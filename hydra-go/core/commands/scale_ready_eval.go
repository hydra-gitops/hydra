package commands

import (
	"fmt"
	"maps"
	"reflect"
	"slices"
	"sort"
	"strings"

	celtypes "github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"hydra-gitops.org/hydra/hydra-go/base/errors"
	"hydra-gitops.org/hydra/hydra-go/base/log"
	"hydra-gitops.org/hydra/hydra-go/core/cel"
	"hydra-gitops.org/hydra/hydra-go/core/entity"
	"hydra-gitops.org/hydra/hydra-go/core/hydra"
	"hydra-gitops.org/hydra/hydra-go/core/types"
)

// Ready rule outcome for scale status / scale-up gating.
const (
	ClusterScaleReadyReady    = "ready"
	ClusterScaleReadyNotReady = "not_ready"
)

// readyRule is one named predicate + CEL list (after merge / builtins).
// Each CEL expression must return: null = omit; "" or [] = pass; non-empty string or list of non-empty strings = failure reasons.
type readyRule struct {
	name      string
	predicate types.CelPredicate
	cel       []types.CelExpression
}

type compiledReadyRule struct {
	name      string
	predicate cel.Predicate
	exprs     []cel.Expression
}

// ReadyEvaluator evaluates global.hydra.ready rules (plus built-in defaults).
type ReadyEvaluator struct {
	rules []compiledReadyRule
	env   cel.Env
}

// ReadyEvaluatorFromHydra builds a compiled evaluator from merged Hydra values and scale map (for builtins).
// live must carry live objects under liveKey so clusterEntities() / involvedObjectEvents(...) match the cluster inventory.
func ReadyEvaluatorFromHydra(
	h hydra.Hydra,
	networkMode types.HelmNetworkMode,
	scaleMap map[types.GVKString]types.HydraScaleGroup,
	live entity.Entities,
	liveKey types.EntityKeyUnstructured,
) (*ReadyEvaluator, error) {
	hv, err := hydra.HydraValues(h, networkMode)
	if err != nil {
		return nil, err
	}
	rules := mergeReadyRules(hv, scaleMap)
	return NewReadyEvaluator(rules, live, liveKey)
}

func mergeReadyRules(hv *types.HydraValues, scaleMap map[types.GVKString]types.HydraScaleGroup) []readyRule {
	byName := map[string]readyRule{}
	if hv != nil {
		for n, g := range hv.Ready {
			byName[n] = readyRule{
				name:      n,
				predicate: types.CelPredicate(g.Predicate),
				cel:       toCelExpressions(g.Cel),
			}
		}
	}
	for _, br := range defaultBuiltinReadyRules(scaleMap) {
		if _, exists := byName[br.name]; !exists {
			byName[br.name] = br
		}
	}
	names := make([]string, 0, len(byName))
	for n := range byName {
		names = append(names, n)
	}
	sort.Strings(names)
	out := make([]readyRule, 0, len(names))
	for _, n := range names {
		out = append(out, byName[n])
	}
	return out
}

func toCelExpressions(ss []string) []types.CelExpression {
	out := make([]types.CelExpression, 0, len(ss))
	for _, s := range ss {
		out = append(out, types.CelExpression(s))
	}
	return out
}

func defaultBuiltinReadyRules(scaleMap map[types.GVKString]types.HydraScaleGroup) []readyRule {
	rules := slices.Clone(embeddedStaticBuiltinReadyRules)
	if scaleMap == nil {
		return rules
	}
	for gvkStr, sg := range scaleMap {
		if sg.StatusReadyPath == "" {
			continue
		}
		expr, err := celStringAtStatusPath(sg.StatusReadyPath)
		if err != nil {
			continue
		}
		safeName := strings.Map(func(r rune) rune {
			switch r {
			case '/', ':', '.':
				return '-'
			default:
				return r
			}
		}, string(gvkStr))
		rules = append(rules, readyRule{
			name:      fmt.Sprintf("hydra-builtin-scale-%s", safeName),
			predicate: types.CelPredicate(fmt.Sprintf(`gvk == "%s"`, string(gvkStr))),
			cel:       toCelExpressions([]string{expr}),
		})
	}
	return rules
}

func celStringAtStatusPath(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", fmt.Errorf("empty path")
	}
	parts := strings.Split(path, ".")
	if len(parts) == 0 {
		return "", fmt.Errorf("empty path")
	}
	var b strings.Builder
	b.WriteString("entity")
	for _, p := range parts {
		if p == "" {
			return "", fmt.Errorf("invalid path segment")
		}
		b.WriteString(".")
		b.WriteString(p)
	}
	base := b.String()
	last := parts[len(parts)-1]
	if last == "readyReplicas" {
		return fmt.Sprintf(`(has(%s) && int(%s) > int(0)) ? "" : has(%s) ? "custom workload: readyReplicas at %s is " + string(int(%s)) + " but must be > 0" : "custom workload: readyReplicas missing at status path %s"`, base, base, base, path, base, path), nil
	}
	if strings.HasSuffix(last, "listeners") || strings.HasSuffix(last, "Conditions") || strings.HasSuffix(last, "conditions") {
		return fmt.Sprintf(`(has(%s) && size(%s) > int(0)) ? "" : "custom workload: expected non-empty list at status path %s"`, base, base, path), nil
	}
	return fmt.Sprintf(`(has(%s) && %s != null) ? "" : "custom workload: expected non-null value at status path %s"`, base, base, path), nil
}

// NewReadyEvaluator compiles merged ready rules (sorted by name; first matching predicate wins).
func NewReadyEvaluator(rules []readyRule, live entity.Entities, liveKey types.EntityKeyUnstructured) (*ReadyEvaluator, error) {
	pre, err := cel.NewEnv()
	if err != nil {
		return nil, err
	}
	invOpt, err := cel.ClusterInventorySupport(pre, live, entity.Entities{}, entity.Entities{})
	if err != nil {
		return nil, err
	}
	env, err := cel.NewEnv(invOpt)
	if err != nil {
		return nil, err
	}
	var compiled []compiledReadyRule
	for _, r := range rules {
		predicate := types.CelPredicate(strings.TrimSpace(string(r.predicate)))
		if predicate == "" {
			return nil, log.CreateError(errors.ErrCelCompileFailed, "ready rule {name}: predicate is required",
				log.String("name", r.name))
		}
		pred, err := env.CompilePredicateAt(fmt.Sprintf(`cluster scale ready rule %q · predicate`, r.name), predicate)
		if err != nil {
			return nil, err
		}
		exprs := make([]cel.Expression, 0, len(r.cel))
		for i, c := range r.cel {
			expr := types.CelExpression(strings.TrimSpace(string(c)))
			if expr == "" {
				continue
			}
			ex, err := env.CompileExpressionAt(fmt.Sprintf(`cluster scale ready rule %q · cel[%d]`, r.name, i), expr)
			if err != nil {
				return nil, err
			}
			exprs = append(exprs, ex)
		}
		if len(exprs) == 0 {
			return nil, log.CreateError(errors.ErrCelCompileFailed, "ready rule {name}: at least one non-empty cel expression is required",
				log.String("name", r.name))
		}
		compiled = append(compiled, compiledReadyRule{name: r.name, predicate: pred, exprs: exprs})
	}
	return &ReadyEvaluator{rules: compiled, env: env}, nil
}

// RuleMatched reports whether any rule's predicate matches the entity (live view when present).
func (re *ReadyEvaluator) RuleMatched(e entity.Entity, liveKey types.EntityKeyUnstructured) bool {
	_, ok, _ := re.matchingRule(e, liveKey)
	return ok
}

// ReadyState returns ready / not_ready when a rule matches; otherwise matched=false.
// messages lists human-readable reasons when state is not_ready (empty when ready or on match failure).
func (re *ReadyEvaluator) ReadyState(e entity.Entity, liveKey types.EntityKeyUnstructured) (matched bool, state string, messages []string, err error) {
	cr, ok, err := re.matchingRule(e, liveKey)
	if err != nil || !ok {
		return ok, "", nil, err
	}
	msgs, evalErr := re.evalRuleConjunctionMessages(cr, e, liveKey)
	if evalErr != nil {
		return true, ClusterScaleReadyNotReady, []string{fmt.Sprintf("ready rule evaluation error: %v", evalErr)}, nil
	}
	if len(msgs) == 0 {
		return true, ClusterScaleReadyReady, nil, nil
	}
	return true, ClusterScaleReadyNotReady, msgs, nil
}

// ReadyFromLiveObject evaluates the winning rule using a freshly fetched live object map as `entity`
// (status fields), while keeping identity fields from e for predicate binding.
func (re *ReadyEvaluator) ReadyFromLiveObject(e entity.Entity, liveObject map[string]any, liveKey types.EntityKeyUnstructured) (matched bool, ready bool, messages []string, err error) {
	if re == nil {
		return false, false, nil, nil
	}
	cr, ok, err := re.matchingRule(e, liveKey)
	if err != nil || !ok {
		return ok, false, nil, err
	}
	m := maps.Clone(re.env.EntityToMap(e))
	m["entity"] = liveObject
	msgs, evalErr := re.evalExprsReadyMessagesFromMap(cr.exprs, m)
	if evalErr != nil {
		return true, false, []string{fmt.Sprintf("ready rule evaluation error: %v", evalErr)}, nil
	}
	return true, len(msgs) == 0, msgs, nil
}

func (re *ReadyEvaluator) matchingRule(e entity.Entity, liveKey types.EntityKeyUnstructured) (compiledReadyRule, bool, error) {
	if re == nil {
		return compiledReadyRule{}, false, nil
	}
	for _, cr := range re.rules {
		ok, err := cr.predicate.EvalBool(e, types.MissingKeysAccept)
		if err != nil {
			continue
		}
		if ok {
			return cr, true, nil
		}
	}
	return compiledReadyRule{}, false, nil
}

func (re *ReadyEvaluator) evalRuleConjunctionMessages(cr compiledReadyRule, e entity.Entity, liveKey types.EntityKeyUnstructured) ([]string, error) {
	if u, ok := e.Unstructured(liveKey); ok {
		m := maps.Clone(re.env.EntityToMap(e))
		m["entity"] = u.Object
		return re.evalExprsReadyMessagesFromMap(cr.exprs, m)
	}
	return re.evalExprsReadyMessages(cr.exprs, e)
}

func (re *ReadyEvaluator) evalExprsReadyMessages(exprs []cel.Expression, e entity.Entity) ([]string, error) {
	var out []string
	for _, ex := range exprs {
		v, err := ex.Eval(e)
		if err != nil {
			return nil, err
		}
		parts, err := readyMessagesFromCELValue(ex.Expression(), v)
		if err != nil {
			return nil, err
		}
		out = append(out, parts...)
	}
	return out, nil
}

func (re *ReadyEvaluator) evalExprsReadyMessagesFromMap(exprs []cel.Expression, m map[string]any) ([]string, error) {
	var out []string
	for _, ex := range exprs {
		v, err := ex.EvalFromMap(m)
		if err != nil {
			return nil, err
		}
		parts, err := readyMessagesFromCELValue(ex.Expression(), v)
		if err != nil {
			return nil, err
		}
		out = append(out, parts...)
	}
	return out, nil
}

func readyMessagesFromCELValue(expr types.CelExpression, v ref.Val) ([]string, error) {
	if v == nil || v == celtypes.NullValue {
		return nil, cel.NewExpressionResultTypeError(expr, "string or []string", v)
	}
	raw := v.Value()
	switch x := raw.(type) {
	case nil:
		return nil, cel.NewExpressionResultTypeError(expr, "string or []string", raw)
	case bool:
		return nil, cel.NewExpressionResultTypeError(expr, "string or []string", raw)
	case string:
		if x == "" {
			return nil, nil
		}
		return []string{x}, nil
	case []string:
		return normalizeReadyReasonStrings(x), nil
	case []any:
		return normalizeReadyReasonAnys(x)
	default:
		if s, err := cel.RefValToNative(v, expr, reflect.TypeOf([]string{}), "string or []string"); err == nil {
			return normalizeReadyReasonStrings(s.([]string)), nil
		}
		if s, err := cel.RefValToNative(v, expr, reflect.TypeOf([]any{}), "string or []string"); err == nil {
			return normalizeReadyReasonAnys(s.([]any))
		}
		return nil, cel.NewExpressionResultTypeError(expr, "string or []string", raw)
	}
}

func normalizeReadyReasonStrings(ss []string) []string {
	var out []string
	for _, s := range ss {
		if s != "" {
			out = append(out, s)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func normalizeReadyReasonAnys(ii []any) ([]string, error) {
	out := make([]string, 0, len(ii))
	for _, v := range ii {
		s, ok := v.(string)
		if !ok {
			return nil, fmt.Errorf("global.hydra.ready list element must be string, got %T", v)
		}
		if s != "" {
			out = append(out, s)
		}
	}
	if len(out) == 0 {
		return nil, nil
	}
	return out, nil
}

// TransitiveOutgoingRefReach returns sorted resource ids reachable from start along outgoing ref edges
// (workload depends on neighbor), up to transitiveRefsMaxDistance hops — same depth cap as inspect / refs.
func TransitiveOutgoingRefReach(refs []types.Ref, start types.Id) []types.Id {
	if start == "" {
		return nil
	}
	_, outgoing := TransitiveRefDetailsForID(refs, start)
	ids := make([]types.Id, len(outgoing))
	for i, d := range outgoing {
		ids[i] = d.ID
	}
	slices.Sort(ids)
	return ids
}

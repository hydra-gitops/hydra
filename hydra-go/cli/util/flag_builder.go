package util

import (
	"strings"

	"github.com/spf13/cobra"
)

type FlagBuilder[T any, V FlagValue[T]] struct {
	cmd       *cobra.Command
	name      string
	short     string
	usage     string
	flagValue V
	validate  func(T) error
	preRun    func(T) error
}

func NewFlagBuilder[T any, V FlagValue[T]](
	cmd *cobra.Command,
	flagValue V,
) *FlagBuilder[T, V] {
	return &FlagBuilder[T, V]{cmd: cmd, flagValue: flagValue}
}

func NewStringFlagBuilder[T ~string](
	cmd *cobra.Command,
	value T,
) *FlagBuilder[T, FlagValue[T]] {
	return NewFlagBuilder(
		cmd,
		NewStringFlagValue(value),
	)
}

func NewBoolFlagBuilder[T ~bool](
	cmd *cobra.Command,
	value T,
) *FlagBuilder[T, FlagValue[T]] {
	return NewFlagBuilder(
		cmd,
		NewBoolFlagValue(value),
	)
}

func (b *FlagBuilder[T, V]) Name(name string) *FlagBuilder[T, V] {
	if b.name != "" {
		panic("name already defined")
	}
	b.name = name
	return b
}

func (b *FlagBuilder[T, V]) Short(short string) *FlagBuilder[T, V] {
	if b.short != "" {
		panic("short already defined")
	}
	b.short = short
	return b
}

func (b *FlagBuilder[T, V]) Usage(usage string) *FlagBuilder[T, V] {
	if b.usage != "" {
		panic("usage already defined")
	}
	b.usage = usage
	return b
}

func (b *FlagBuilder[T, V]) Validate(fn func(T) error) *FlagBuilder[T, V] {
	if b.validate != nil {
		panic("validate already defined")
	}
	b.validate = fn
	return b
}
func (b *FlagBuilder[T, V]) PreRun(fn func(T) error) *FlagBuilder[T, V] {
	if b.preRun != nil {
		panic("preRun already defined")
	}
	b.preRun = fn
	return b
}

func (b *FlagBuilder[T, V]) Build() {
	flags := b.cmd.Flags()
	switch t := b.flagValue.Type(); t {
	case FlagTypeBool:
		flags.BoolP(b.name, b.short, true, b.usage)
		f := flags.Lookup(b.name)
		f.DefValue = ""
	case FlagTypeString:
		flags.StringP(b.name, b.short, b.flagValue.String(), b.usage)
	case FlagTypeEnum:
		val := &setterWrapper{
			setter:      b.flagValue.SetString,
			getter:      b.flagValue.String,
			wrappedType: b.name,
		}
		options := b.flagValue.StringValues()
		o := strings.Join(options, "|")
		flags.VarPF(val, b.name, b.short, b.usage+" ("+o+")")
		b.cmd.RegisterFlagCompletionFunc(b.name, func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			return options, cobra.ShellCompDirectiveNoFileComp
		})
	}

	f := flags.Lookup(b.name)
	AddPreRun(b.cmd, func(cmd *cobra.Command, args []string) {
		b.flagValue.SetString(f.Value.String())
	})

	if b.preRun != nil {
		AddPreRunE(b.cmd, func(cmd *cobra.Command, args []string) error {
			v := b.flagValue.Value()
			return b.preRun(v)
		})
	}

	if b.validate != nil {
		AddPreRunE(b.cmd, func(cmd *cobra.Command, args []string) error {
			if f.Changed {
				v := b.flagValue.Value()
				return b.validate(v)
			}
			return nil
		})
	}
}

type setterWrapper struct {
	setter      func(string) error
	getter      func() string
	wrappedType string
}

func (s *setterWrapper) Set(v string) error {
	return s.setter(v)
}

func (s *setterWrapper) String() string {
	return s.getter()
}

func (s *setterWrapper) Type() string {
	return s.wrappedType
}

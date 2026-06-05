# hydra-go/base

Generic utilities and helper classes without domain-specific logic.

## Packages

### cache

Generic thread-safe cache with lazy loading.

```go
cache := cache.NewCache[KeyType, ValueType]("name", false, nil)
value, err := cache.GetOrLoad(key, func() (ValueType, error) {
    return loadValue()
})
```

### colors

ANSI color constants for terminal output.

```go
fmt.Print(colors.Green.String() + "Success" + colors.Reset.String())
diff := colors.ColorDiff(diffString)
```

### errors

Error interface with error IDs for structured error handling.

```go
type Error interface {
    error
    ErrorId() ErrorId
}

if errors.ErrKeyNotFound.MatchesError(err) {
    // Handle specific error
}
```

### log

Structured logging with slog integration.

- `color_handler.go` - Colored console output handler
- `context.go` - Context-aware logging
- `debug.go` - Debug logging utilities
- `error.go` - Error logging with lazy evaluation
- `fatal.go` - Fatal error handling
- `format_handler.go` - Message formatting with placeholders
- `transformer_handler.go` - Log message transformation
- `writer.go` - Custom log writers

```go
log.Configure(log.Config{
    Level:  slog.LevelInfo,
    Colors: &log.DefaultColors,
})

err := log.CreateError(errors.ErrInternalError, "message with {key}", slog.String("key", value))
```

### types

Generic types without domain logic.

- `EnumType[T]` - Interface for type-safe enums
- `YamlString` - Type alias for YAML strings
- `ValuesMap` - Type alias for `map[string]any`

### utils

Generic helper functions.

- `Clone[T]` - Shallow clone of a pointer
- `Ptr[T]` - Create pointer to value
- `FileUriToPath` - Convert file URI to path
- `EnvWrapper` - Temporarily set environment variable

```go
ptr := utils.Ptr(value)
clone := utils.Clone(ptr)
restore := utils.EnvWrapper("VAR", "value")
defer restore()
```

## Design Principles

- **No Hydra-specific logic** - Purely generic utilities
- **No Cobra dependencies** - Framework-agnostic
- **No dependencies on core or cli** - Base module without upstream dependencies
- **No external dependencies** - Uses only the Go standard library

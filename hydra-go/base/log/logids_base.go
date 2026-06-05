package log

// Private package-level LogIds for base module
var (
	baseCache  = base.Child("cache")
	baseColors = base.Child("colors")
	baseErrors = base.Child("errors")
	baseLog    = base.Child("log")
	baseTypes  = base.Child("types")
	baseUtils  = base.Child("utils")
)

// Private struct-level LogIds for base/cache
var baseCacheCache = baseCache.Child("Cache")

// Private struct-level LogIds for base/log
var baseLogLogger = baseLog.Child("Logger")

// Public accessor functions returning clones

// BaseCache returns a clone of the base/cache package LogId.
func BaseCache() LogId { return baseCache.Clone() }

// BaseColors returns a clone of the base/colors package LogId.
func BaseColors() LogId { return baseColors.Clone() }

// BaseErrors returns a clone of the base/errors package LogId.
func BaseErrors() LogId { return baseErrors.Clone() }

// BaseLog returns a clone of the base/log package LogId.
func BaseLog() LogId { return baseLog.Clone() }

// BaseTypes returns a clone of the base/types package LogId.
func BaseTypes() LogId { return baseTypes.Clone() }

// BaseUtils returns a clone of the base/utils package LogId.
func BaseUtils() LogId { return baseUtils.Clone() }

// BaseCacheCache returns a clone of the base/cache.Cache struct LogId.
func BaseCacheCache() LogId { return baseCacheCache.Clone() }

// BaseLogLogger returns a clone of the base/log.Logger struct LogId.
func BaseLogLogger() LogId { return baseLogLogger.Clone() }

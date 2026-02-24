package repl

var sourceCommands = []string{
	"FROM", "ROW", "SHOW",
}

var processingCommands = []string{
	"WHERE", "EVAL", "STATS", "SORT", "LIMIT", "KEEP", "DROP", "RENAME",
	"DISSECT", "GROK", "ENRICH", "LOOKUP JOIN", "MV_EXPAND",
	"INLINE STATS", "CHANGE_POINT", "FORK", "FUSE", "SAMPLE",
	"RERANK", "COMPLETION",
}

var clauses = []string{
	"BY", "AS", "ON", "WITH", "METADATA",
	"ASC", "DESC", "NULLS FIRST", "NULLS LAST",
	"AND", "OR", "NOT", "IN", "LIKE", "RLIKE",
	"IS NULL", "IS NOT NULL",
	"TRUE", "FALSE", "NULL", "INFO",
}

var aggregationFunctions = []string{
	"ABSENT", "AVG", "COUNT", "COUNT_DISTINCT",
	"FIRST", "LAST", "MAX", "MEDIAN",
	"MEDIAN_ABSOLUTE_DEVIATION", "MIN", "PERCENTILE",
	"PRESENT", "SAMPLE", "ST_CENTROID_AGG", "ST_EXTENT_AGG",
	"STD_DEV", "SUM", "TOP", "VALUES", "VARIANCE", "WEIGHTED_AVG",
}

var timeSeriesAggFunctions = []string{
	"ABSENT_OVER_TIME", "AVG_OVER_TIME", "COUNT_OVER_TIME",
	"COUNT_DISTINCT_OVER_TIME", "DELTA", "DERIV",
	"FIRST_OVER_TIME", "IDELTA", "INCREASE", "IRATE",
	"LAST_OVER_TIME", "MAX_OVER_TIME", "MIN_OVER_TIME",
	"PERCENTILE_OVER_TIME", "PRESENT_OVER_TIME", "RATE",
	"STDDEV_OVER_TIME", "SUM_OVER_TIME", "VARIANCE_OVER_TIME",
}

var groupingFunctions = []string{
	"BUCKET", "TBUCKET", "CATEGORIZE",
}

var conditionalFunctions = []string{
	"CASE", "COALESCE", "GREATEST", "LEAST",
	"CLAMP", "CLAMP_MIN", "CLAMP_MAX",
}

var dateTimeFunctions = []string{
	"DATE_DIFF", "DATE_EXTRACT", "DATE_FORMAT", "DATE_PARSE",
	"DATE_TRUNC", "DAY_NAME", "MONTH_NAME", "NOW", "TRANGE",
}

var stringFunctions = []string{
	"BIT_LENGTH", "BYTE_LENGTH", "CHUNK", "CONCAT", "CONTAINS",
	"ENDS_WITH", "FROM_BASE64", "HASH", "LEFT", "LENGTH",
	"LOCATE", "LTRIM", "MD5", "REPEAT", "REPLACE", "REVERSE",
	"RIGHT", "RTRIM", "SHA1", "SHA256", "SPACE", "SPLIT",
	"STARTS_WITH", "SUBSTRING", "TO_BASE64", "TO_LOWER",
	"TO_UPPER", "TRIM", "URL_DECODE", "URL_ENCODE",
	"URL_ENCODE_COMPONENT",
}

var mathFunctions = []string{
	"ABS", "ACOS", "ACOSH", "ASIN", "ASINH", "ATAN", "ATAN2",
	"ATANH", "CBRT", "CEIL", "COPY_SIGN", "COS", "COSH", "E",
	"EXP", "FLOOR", "HYPOT", "LOG", "LOG10", "PI", "POW",
	"ROUND", "ROUND_TO", "SCALB", "SIGNUM", "SIN", "SINH",
	"SQRT", "TAN", "TANH", "TAU",
}

var ipFunctions = []string{
	"CIDR_MATCH", "IP_PREFIX",
}

var spatialFunctions = []string{
	"ST_CONTAINS", "ST_DISJOINT", "ST_DISTANCE", "ST_ENVELOPE",
	"ST_GEOHASH", "ST_GEOHEX", "ST_GEOTILE", "ST_INTERSECTS",
	"ST_NPOINTS", "ST_SIMPLIFY", "ST_WITHIN",
	"ST_X", "ST_XMAX", "ST_XMIN", "ST_Y", "ST_YMAX", "ST_YMIN",
}

var searchFunctions = []string{
	"DECAY", "KQL", "MATCH", "MATCH_PHRASE", "QSTR", "SCORE",
	"TOP_SNIPPETS",
}

var denseVectorFunctions = []string{
	"KNN", "TEXT_EMBEDDING", "V_COSINE", "V_DOT_PRODUCT",
	"V_HAMMING", "V_L1_NORM", "V_L2_NORM",
}

var typeConversionFunctions = []string{
	"TO_BOOLEAN", "TO_CARTESIANPOINT", "TO_CARTESIANSHAPE",
	"TO_DATEPERIOD", "TO_DATETIME", "TO_DATE_NANOS",
	"TO_DEGREES", "TO_DENSE_VECTOR", "TO_DOUBLE",
	"TO_GEOPOINT", "TO_GEOSHAPE", "TO_INTEGER", "TO_IP",
	"TO_LONG", "TO_RADIANS", "TO_STRING", "TO_TIMEDURATION",
	"TO_UNSIGNED_LONG", "TO_VERSION",
}

var multivalueFunctions = []string{
	"MV_APPEND", "MV_AVG", "MV_CONCAT", "MV_CONTAINS",
	"MV_COUNT", "MV_DEDUPE", "MV_FIRST", "MV_INTERSECTION",
	"MV_INTERSECTS", "MV_LAST", "MV_MAX", "MV_MEDIAN",
	"MV_MEDIAN_ABSOLUTE_DEVIATION", "MV_MIN", "MV_PERCENTILE",
	"MV_PSERIES_WEIGHTED_SUM", "MV_SLICE", "MV_SORT", "MV_SUM",
	"MV_UNION", "MV_ZIP",
}

// allFunctions is the combined list of all ES|QL functions.
var allFunctions []string

// allKeywords is the combined list of all commands and clauses.
var allKeywords []string

func init() {
	allFunctions = make([]string, 0, 200)
	allFunctions = append(allFunctions, aggregationFunctions...)
	allFunctions = append(allFunctions, timeSeriesAggFunctions...)
	allFunctions = append(allFunctions, groupingFunctions...)
	allFunctions = append(allFunctions, conditionalFunctions...)
	allFunctions = append(allFunctions, dateTimeFunctions...)
	allFunctions = append(allFunctions, stringFunctions...)
	allFunctions = append(allFunctions, mathFunctions...)
	allFunctions = append(allFunctions, ipFunctions...)
	allFunctions = append(allFunctions, spatialFunctions...)
	allFunctions = append(allFunctions, searchFunctions...)
	allFunctions = append(allFunctions, denseVectorFunctions...)
	allFunctions = append(allFunctions, typeConversionFunctions...)
	allFunctions = append(allFunctions, multivalueFunctions...)

	allKeywords = make([]string, 0, 50)
	allKeywords = append(allKeywords, sourceCommands...)
	allKeywords = append(allKeywords, processingCommands...)
	allKeywords = append(allKeywords, clauses...)
}

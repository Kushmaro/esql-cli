package repl

var sourceCommands = []string{
	"FROM", "ROW", "SHOW", "TS", "PROMQL",
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

// funcSignatures maps function names to their brief call signatures.
var funcSignatures = map[string]string{
	// Aggregation
	"ABSENT":                     "ABSENT(field)",
	"AVG":                        "AVG(number)",
	"COUNT":                      "COUNT(*|field)",
	"COUNT_DISTINCT":             "COUNT_DISTINCT(field [, precision])",
	"FIRST":                      "FIRST(field [, sort_field])",
	"LAST":                       "LAST(field [, sort_field])",
	"MAX":                        "MAX(field)",
	"MEDIAN":                     "MEDIAN(number)",
	"MEDIAN_ABSOLUTE_DEVIATION":  "MEDIAN_ABSOLUTE_DEVIATION(number)",
	"MIN":                        "MIN(field)",
	"PERCENTILE":                 "PERCENTILE(field, percentile)",
	"PRESENT":                    "PRESENT(field)",
	"SAMPLE":                     "SAMPLE(field)",
	"ST_CENTROID_AGG":            "ST_CENTROID_AGG(point)",
	"ST_EXTENT_AGG":              "ST_EXTENT_AGG(point)",
	"STD_DEV":                    "STD_DEV(number)",
	"SUM":                        "SUM(number)",
	"TOP":                        "TOP(field, limit, order)",
	"VALUES":                     "VALUES(field)",
	"VARIANCE":                   "VARIANCE(number)",
	"WEIGHTED_AVG":               "WEIGHTED_AVG(value, weight)",
	// Time series aggregation
	"ABSENT_OVER_TIME":           "ABSENT_OVER_TIME(field)",
	"AVG_OVER_TIME":              "AVG_OVER_TIME(number)",
	"COUNT_OVER_TIME":            "COUNT_OVER_TIME(*|field)",
	"COUNT_DISTINCT_OVER_TIME":   "COUNT_DISTINCT_OVER_TIME(field)",
	"DELTA":                      "DELTA(number)",
	"DERIV":                      "DERIV(number)",
	"FIRST_OVER_TIME":            "FIRST_OVER_TIME(field)",
	"IDELTA":                     "IDELTA(number)",
	"INCREASE":                   "INCREASE(number)",
	"IRATE":                      "IRATE(number)",
	"LAST_OVER_TIME":             "LAST_OVER_TIME(field)",
	"MAX_OVER_TIME":              "MAX_OVER_TIME(field)",
	"MIN_OVER_TIME":              "MIN_OVER_TIME(field)",
	"PERCENTILE_OVER_TIME":       "PERCENTILE_OVER_TIME(field, percentile)",
	"PRESENT_OVER_TIME":          "PRESENT_OVER_TIME(field)",
	"RATE":                       "RATE(number)",
	"STDDEV_OVER_TIME":           "STDDEV_OVER_TIME(number)",
	"SUM_OVER_TIME":              "SUM_OVER_TIME(number)",
	"VARIANCE_OVER_TIME":         "VARIANCE_OVER_TIME(number)",
	// Grouping
	"BUCKET":                     "BUCKET(field, buckets [, from, to])",
	"TBUCKET":                    "TBUCKET(field, interval)",
	"CATEGORIZE":                 "CATEGORIZE(field)",
	// Conditional
	"CASE":                       "CASE(condition, value [, ...])",
	"COALESCE":                   "COALESCE(value [, ...])",
	"GREATEST":                   "GREATEST(value [, ...])",
	"LEAST":                      "LEAST(value [, ...])",
	"CLAMP":                      "CLAMP(value, min, max)",
	"CLAMP_MIN":                  "CLAMP_MIN(value, min)",
	"CLAMP_MAX":                  "CLAMP_MAX(value, max)",
	// Date/time
	"DATE_DIFF":                  "DATE_DIFF(unit, start, end)",
	"DATE_EXTRACT":               "DATE_EXTRACT(part, date)",
	"DATE_FORMAT":                "DATE_FORMAT(date [, format])",
	"DATE_PARSE":                 "DATE_PARSE(format, string)",
	"DATE_TRUNC":                 "DATE_TRUNC(interval, date)",
	"DAY_NAME":                   "DAY_NAME(date)",
	"MONTH_NAME":                 "MONTH_NAME(date)",
	"NOW":                        "NOW()",
	"TRANGE":                     "TRANGE(start, end)",
	// String
	"BIT_LENGTH":                 "BIT_LENGTH(string)",
	"BYTE_LENGTH":                "BYTE_LENGTH(string)",
	"CHUNK":                      "CHUNK(string, max_length [, overlap])",
	"CONCAT":                     "CONCAT(string [, ...])",
	"CONTAINS":                   "CONTAINS(string, substring)",
	"ENDS_WITH":                  "ENDS_WITH(string, suffix)",
	"FROM_BASE64":                "FROM_BASE64(string)",
	"HASH":                       "HASH(algorithm, value)",
	"LEFT":                       "LEFT(string, length)",
	"LENGTH":                     "LENGTH(string)",
	"LOCATE":                     "LOCATE(string, substring [, start])",
	"LTRIM":                      "LTRIM(string)",
	"MD5":                        "MD5(value)",
	"REPEAT":                     "REPEAT(string, count)",
	"REPLACE":                    "REPLACE(string, search, replacement)",
	"REVERSE":                    "REVERSE(string)",
	"RIGHT":                      "RIGHT(string, length)",
	"RTRIM":                      "RTRIM(string)",
	"SHA1":                       "SHA1(value)",
	"SHA256":                     "SHA256(value)",
	"SPACE":                      "SPACE(count)",
	"SPLIT":                      "SPLIT(string, delimiter)",
	"STARTS_WITH":                "STARTS_WITH(string, prefix)",
	"SUBSTRING":                  "SUBSTRING(string, start [, length])",
	"TO_BASE64":                  "TO_BASE64(value)",
	"TO_LOWER":                   "TO_LOWER(string)",
	"TO_UPPER":                   "TO_UPPER(string)",
	"TRIM":                       "TRIM(string)",
	"URL_DECODE":                 "URL_DECODE(string)",
	"URL_ENCODE":                 "URL_ENCODE(string)",
	"URL_ENCODE_COMPONENT":       "URL_ENCODE_COMPONENT(string)",
	// Math
	"ABS":                        "ABS(number)",
	"ACOS":                       "ACOS(number)",
	"ACOSH":                      "ACOSH(number)",
	"ASIN":                       "ASIN(number)",
	"ASINH":                      "ASINH(number)",
	"ATAN":                       "ATAN(number)",
	"ATAN2":                      "ATAN2(y, x)",
	"ATANH":                      "ATANH(number)",
	"CBRT":                       "CBRT(number)",
	"CEIL":                       "CEIL(number)",
	"COPY_SIGN":                  "COPY_SIGN(magnitude, sign)",
	"COS":                        "COS(angle)",
	"COSH":                       "COSH(number)",
	"E":                          "E()",
	"EXP":                        "EXP(number)",
	"FLOOR":                      "FLOOR(number)",
	"HYPOT":                      "HYPOT(x, y)",
	"LOG":                        "LOG(base, value)",
	"LOG10":                      "LOG10(number)",
	"PI":                         "PI()",
	"POW":                        "POW(base, exponent)",
	"ROUND":                      "ROUND(number [, decimals])",
	"ROUND_TO":                   "ROUND_TO(number, precision)",
	"SCALB":                      "SCALB(number, scale)",
	"SIGNUM":                     "SIGNUM(number)",
	"SIN":                        "SIN(angle)",
	"SINH":                       "SINH(number)",
	"SQRT":                       "SQRT(number)",
	"TAN":                        "TAN(angle)",
	"TANH":                       "TANH(number)",
	"TAU":                        "TAU()",
	// IP
	"CIDR_MATCH":                 "CIDR_MATCH(ip, cidr [, ...])",
	"IP_PREFIX":                  "IP_PREFIX(ip, prefix_v4, prefix_v6)",
	// Spatial
	"ST_CONTAINS":                "ST_CONTAINS(geometry, geometry)",
	"ST_DISJOINT":                "ST_DISJOINT(geometry, geometry)",
	"ST_DISTANCE":                "ST_DISTANCE(point, point)",
	"ST_ENVELOPE":                "ST_ENVELOPE(geometry)",
	"ST_GEOHASH":                 "ST_GEOHASH(point, precision)",
	"ST_GEOHEX":                  "ST_GEOHEX(point, precision)",
	"ST_GEOTILE":                 "ST_GEOTILE(point, zoom)",
	"ST_INTERSECTS":              "ST_INTERSECTS(geometry, geometry)",
	"ST_NPOINTS":                 "ST_NPOINTS(geometry)",
	"ST_SIMPLIFY":                "ST_SIMPLIFY(geometry, tolerance)",
	"ST_WITHIN":                  "ST_WITHIN(geometry, geometry)",
	"ST_X":                       "ST_X(point)",
	"ST_XMAX":                    "ST_XMAX(geometry)",
	"ST_XMIN":                    "ST_XMIN(geometry)",
	"ST_Y":                       "ST_Y(point)",
	"ST_YMAX":                    "ST_YMAX(geometry)",
	"ST_YMIN":                    "ST_YMIN(geometry)",
	// Search
	"DECAY":                      "DECAY(function, field, origin, scale [, offset, decay])",
	"KQL":                        "KQL(query)",
	"MATCH":                      "MATCH(field, query)",
	"MATCH_PHRASE":               "MATCH_PHRASE(field, query)",
	"QSTR":                       "QSTR(query)",
	"SCORE":                      "SCORE()",
	"TOP_SNIPPETS":               "TOP_SNIPPETS(field, size [, query])",
	// Dense vector
	"KNN":                        "KNN(field, k, query_vector)",
	"TEXT_EMBEDDING":             "TEXT_EMBEDDING(model, text)",
	"V_COSINE":                   "V_COSINE(vector, vector)",
	"V_DOT_PRODUCT":              "V_DOT_PRODUCT(vector, vector)",
	"V_HAMMING":                  "V_HAMMING(vector, vector)",
	"V_L1_NORM":                  "V_L1_NORM(vector, vector)",
	"V_L2_NORM":                  "V_L2_NORM(vector, vector)",
	// Type conversion
	"TO_BOOLEAN":                 "TO_BOOLEAN(value)",
	"TO_CARTESIANPOINT":          "TO_CARTESIANPOINT(value)",
	"TO_CARTESIANSHAPE":          "TO_CARTESIANSHAPE(value)",
	"TO_DATEPERIOD":              "TO_DATEPERIOD(value)",
	"TO_DATETIME":                "TO_DATETIME(value)",
	"TO_DATE_NANOS":              "TO_DATE_NANOS(value)",
	"TO_DEGREES":                 "TO_DEGREES(number)",
	"TO_DENSE_VECTOR":            "TO_DENSE_VECTOR(value)",
	"TO_DOUBLE":                  "TO_DOUBLE(value)",
	"TO_GEOPOINT":                "TO_GEOPOINT(value)",
	"TO_GEOSHAPE":                "TO_GEOSHAPE(value)",
	"TO_INTEGER":                 "TO_INTEGER(value)",
	"TO_IP":                      "TO_IP(value)",
	"TO_LONG":                    "TO_LONG(value)",
	"TO_RADIANS":                 "TO_RADIANS(number)",
	"TO_STRING":                  "TO_STRING(value)",
	"TO_TIMEDURATION":            "TO_TIMEDURATION(value)",
	"TO_UNSIGNED_LONG":           "TO_UNSIGNED_LONG(value)",
	"TO_VERSION":                 "TO_VERSION(value)",
	// Multivalue
	"MV_APPEND":                  "MV_APPEND(field, field)",
	"MV_AVG":                     "MV_AVG(number)",
	"MV_CONCAT":                  "MV_CONCAT(field, delimiter)",
	"MV_CONTAINS":                "MV_CONTAINS(field, value)",
	"MV_COUNT":                   "MV_COUNT(field)",
	"MV_DEDUPE":                  "MV_DEDUPE(field)",
	"MV_FIRST":                   "MV_FIRST(field)",
	"MV_INTERSECTION":            "MV_INTERSECTION(field, field)",
	"MV_INTERSECTS":              "MV_INTERSECTS(field, field)",
	"MV_LAST":                    "MV_LAST(field)",
	"MV_MAX":                     "MV_MAX(field)",
	"MV_MEDIAN":                  "MV_MEDIAN(number)",
	"MV_MEDIAN_ABSOLUTE_DEVIATION": "MV_MEDIAN_ABSOLUTE_DEVIATION(number)",
	"MV_MIN":                     "MV_MIN(field)",
	"MV_PERCENTILE":              "MV_PERCENTILE(field, percentile)",
	"MV_PSERIES_WEIGHTED_SUM":    "MV_PSERIES_WEIGHTED_SUM(number, p)",
	"MV_SLICE":                   "MV_SLICE(field, start [, end])",
	"MV_SORT":                    "MV_SORT(field [, order])",
	"MV_SUM":                     "MV_SUM(number)",
	"MV_UNION":                   "MV_UNION(field, field)",
	"MV_ZIP":                     "MV_ZIP(left, right [, delimiter])",
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

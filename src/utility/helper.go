package utility

import (
	"bytes"
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"os"
	"regexp"
	"runtime"
	"skycrypt/src/constants"
	"skycrypt/src/db"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

var (
	colorCodeRegex = regexp.MustCompile("§[0-9a-fk-or]")
	nonAsciiRegex  = regexp.MustCompile(`[^\x00-\x7F]`)
	variableRegex  = regexp.MustCompile(`\{(\w+)\}`) // Moved from ReplaceVariables
)

var titleCaser = cases.Title(language.English)

type errorCache struct {
	lastSent time.Time
	count    int
}

var (
	errorCacheMutex sync.RWMutex
	errorCacheMap   = make(map[string]*errorCache)
	cacheDuration   = 15 * time.Minute
)

var (
	skinHashCacheMutex sync.RWMutex
	skinHashCache      = make(map[string]string)
	base64Encodings    = []*base64.Encoding{
		base64.RawStdEncoding, // Standard base64 without padding
		base64.StdEncoding,    // Standard base64 with padding
		base64.RawURLEncoding, // URL-safe base64 without padding
		base64.URLEncoding,    // URL-safe base64 with padding
	}
)

var httpClient = &http.Client{
	Timeout: 10 * time.Second,
}

var bufferPool = sync.Pool{
	New: func() interface{} {
		return bytes.NewBuffer(make([]byte, 0, 1024))
	},
}

var rarityMap = func() map[string]int {
	m := make(map[string]int)
	for i, r := range constants.RARITIES {
		m[strings.ToLower(r)] = i
	}
	return m
}()

func GetRawLore(text string) string {
	return colorCodeRegex.ReplaceAllString(text, "")
}

func RemoveNonAscii(text string) string {
	return nonAsciiRegex.ReplaceAllString(text, "")
}

func Contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func GetLastValue(m map[int]int) int {
	maxKey := 0
	for key := range m {
		if key > maxKey {
			maxKey = key
		}
	}
	return m[maxKey]
}

func Min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func Max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func TitleCase(s string) string {
	if !strings.ContainsAny(s, "_-") {
		return titleCaser.String(s)
	}

	parts := strings.FieldsFunc(s, func(r rune) bool {
		return r == '_' || r == '-'
	})

	var builder strings.Builder
	builder.Grow(len(s))

	for i, part := range parts {
		if i > 0 {
			builder.WriteByte(' ')
		}
		builder.WriteString(titleCaser.String(part))
	}

	return builder.String()
}

func ParseInt(n string) (int, error) {
	return strconv.Atoi(n)
}

func RarityNameToInt(rarity string) int {
	if idx, ok := rarityMap[strings.ToLower(rarity)]; ok {
		return idx
	}
	return 0
}

func FormatNumber(n any) string {
	var value float64
	switch v := n.(type) {
	case int:
		value = float64(v)
	case float64:
		value = v
	case float32:
		value = float64(v)
	case int64:
		value = float64(v)
	default:
		return "0"
	}

	abs := math.Abs(value)

	switch {
	case abs >= 1e9:
		return formatWithSuffix(value/1e9, "B")
	case abs >= 1e6:
		return formatWithSuffix(value/1e6, "M")
	case abs >= 1e3:
		return formatWithSuffix(value/1e3, "K")
	default:
		if value == float64(int(value)) {
			return strconv.Itoa(int(value))
		}
		return strconv.FormatFloat(Round(value, 2), 'f', -1, 64)
	}
}

func formatWithSuffix(value float64, suffix string) string {
	rounded := Round(value, 2)
	if rounded == float64(int(rounded)) {
		return strconv.Itoa(int(rounded)) + suffix
	}
	return strconv.FormatFloat(rounded, 'f', -1, 64) + suffix
}

func AddCommas(n int) string {
	if n < 1000 {
		return strconv.Itoa(n)
	}

	s := strconv.Itoa(n)
	length := len(s)
	commaCount := (length - 1) / 3

	var builder strings.Builder
	builder.Grow(length + commaCount)

	startOffset := length % 3
	if startOffset == 0 {
		startOffset = 3
	}

	builder.WriteString(s[:startOffset])
	for i := startOffset; i < length; i += 3 {
		builder.WriteByte(',')
		builder.WriteString(s[i : i+3])
	}

	return builder.String()
}

func ParseTimestamp(timestamp string) int {
	t, err := time.Parse("1/2/06 3:04 PM", timestamp)
	if err != nil {
		return 0
	}
	return int(t.Unix())
}

func Every[T any](slice []T, predicate func(T) bool) bool {
	for _, item := range slice {
		if !predicate(item) {
			return false
		}
	}
	return true
}

func IndexOf(slice []string, item string) int {
	for i, v := range slice {
		if v == item {
			return i
		}
	}
	return -1
}

func GetSkinHash(base64String string) string {
	if base64String == "" {
		return ""
	}

	skinHashCacheMutex.RLock()
	if cached, exists := skinHashCache[base64String]; exists {
		skinHashCacheMutex.RUnlock()
		return cached
	}
	skinHashCacheMutex.RUnlock()

	result := computeSkinHash(base64String)

	skinHashCacheMutex.Lock()
	skinHashCache[base64String] = result
	skinHashCacheMutex.Unlock()

	return result
}

type skinTextureData struct {
	Textures struct {
		SKIN struct {
			URL string `json:"url"`
		} `json:"SKIN"`
	} `json:"textures"`
}

func computeSkinHash(base64String string) string {
	var data []byte

	for _, encoding := range base64Encodings {
		var err error
		data, err = encoding.DecodeString(base64String)
		if err == nil {
			break
		}
	}

	if data == nil {
		return ""
	}

	var jsonData skinTextureData
	if err := json.Unmarshal(data, &jsonData); err != nil {
		return ""
	}

	url := jsonData.Textures.SKIN.URL
	if url == "" {
		return ""
	}

	lastSlash := strings.LastIndex(url, "/")
	if lastSlash == -1 || lastSlash == len(url)-1 {
		return ""
	}
	return url[lastSlash+1:]
}

func Round(value float64, precision int) float64 {
	if precision < 0 {
		return value
	}
	pow := math.Pow(10, float64(precision))
	return math.Round(value*pow) / pow
}

func ReplaceVariables(template string, variables map[string]float64) string {
	return variableRegex.ReplaceAllStringFunc(template, func(match string) string {
		name := match[1 : len(match)-1] // Faster than strings.Trim

		value, exists := variables[name]
		if !exists {
			return match
		}

		if _, err := strconv.ParseFloat(name, 64); err != nil {
			if intValue := int(value); intValue > 0 {
				return "+" + strconv.Itoa(intValue)
			}
		}

		return strconv.Itoa(int(math.Abs(value)))
	})
}

func CompareInts(a, b int) int {
	if a < b {
		return -1
	} else if a > b {
		return 1
	}
	return 0
}

func CompareStrings(a, b string) int {
	if a < b {
		return -1
	} else if a > b {
		return 1
	}
	return 0
}

func CompareBooleans(a, b bool) int {
	if a == b {
		return 0
	} else if a {
		return 1
	}
	return -1
}

func Filter[T any](slice []T, predicate func(T) bool) []T {
	result := make([]T, 0, len(slice)/2) // Estimate half will match
	for _, item := range slice {
		if predicate(item) {
			result = append(result, item)
		}
	}
	return result
}

func SortBy[T any](slice []T, compare func(T, T) int) []T {
	sort.Slice(slice, func(i, j int) bool {
		return compare(slice[i], slice[j]) < 0
	})
	return slice
}

func Sum(slice []float64) float64 {
	var total float64
	for _, value := range slice {
		total += value
	}
	return total
}

func RoundFloat(value float64, precision int) float64 {
	return Round(value, precision)
}

func SortInts(slice []int) []int {
	sort.Ints(slice)
	return slice
}

func SumInt(slice []int) int {
	total := 0
	for _, value := range slice {
		total += value
	}
	return total
}

func SortSlice[T any](slice []T, less func(i, j int) bool) {
	sort.Slice(slice, less)
}

func SendWebhook(endpoint string, err interface{}, stack []byte) {
	webhookURL := os.Getenv("DISCORD_WEBHOOK")
	if webhookURL == "" {
		return
	}

	errorStr := fmt.Sprintf("%v", err)
	errorHash := generateErrorHash(endpoint, errorStr)

	if !shouldSendError(errorHash) {
		return
	}

	pc, file, line, ok := runtime.Caller(1)
	var callerInfo string
	if ok {
		fn := runtime.FuncForPC(pc)
		callerInfo = fmt.Sprintf("%s:%d in %s", file, line, fn.Name())
	} else {
		callerInfo = "Unknown caller"
	}

	stackStr := string(stack)
	if len(stackStr) > 800 {
		stackStr = stackStr[:800] + "\n... (truncated)"
	}

	// Simplify file path
	cleanFilePath := callerInfo
	if lastSlash := strings.LastIndex(callerInfo, "/"); lastSlash != -1 {
		if secondLast := strings.LastIndex(callerInfo[:lastSlash], "/"); secondLast != -1 {
			cleanFilePath = callerInfo[secondLast+1:]
		}
	}

	if len(errorStr) > 100 {
		errorStr = errorStr[:100] + "..."
	}

	errorCount := getErrorCount(errorHash)
	var countText string
	if errorCount > 1 {
		countText = fmt.Sprintf(" (occurred %d times)", errorCount)
	}

	embed := map[string]interface{}{
		"color": 0xFF3B30,
		"fields": []map[string]interface{}{
			{"name": "Error Details" + countText, "value": "```\n" + errorStr + "\n```", "inline": false},
			{"name": "Endpoint", "value": "`" + endpoint + "`", "inline": true},
			{"name": "Occurred", "value": fmt.Sprintf("<t:%d:R>", time.Now().Unix()), "inline": true},
			{"name": "Location", "value": "`" + cleanFilePath + "`", "inline": false},
			{"name": "Stack Trace", "value": "```go\n" + stackStr + "\n```", "inline": false},
		},
	}

	payload := map[string]interface{}{
		"username": "SkyCrypt Monitor",
		"embeds":   []map[string]interface{}{embed},
	}

	// Use pooled buffer
	buf := bufferPool.Get().(*bytes.Buffer)
	buf.Reset()
	defer bufferPool.Put(buf)

	if err := json.NewEncoder(buf).Encode(payload); err != nil {
		return
	}

	// Use shared HTTP client
	resp, httpErr := httpClient.Post(webhookURL, "application/json", buf)
	if httpErr != nil {
		return
	}
	defer resp.Body.Close()
}

func generateErrorHash(endpoint, errorStr string) string {
	hash := md5.Sum([]byte(endpoint + ":" + errorStr))
	return hex.EncodeToString(hash[:])
}

func shouldSendError(errorHash string) bool {
	errorCacheMutex.Lock()
	defer errorCacheMutex.Unlock()

	now := time.Now()
	cache, exists := errorCacheMap[errorHash]

	if !exists {
		errorCacheMap[errorHash] = &errorCache{lastSent: now, count: 1}
		return true
	}

	cache.count++

	if now.Sub(cache.lastSent) >= cacheDuration {
		cache.lastSent = now
		return true
	}

	return false
}

func getErrorCount(errorHash string) int {
	errorCacheMutex.RLock()
	defer errorCacheMutex.RUnlock()

	if cache, exists := errorCacheMap[errorHash]; exists {
		return cache.count
	}
	return 1
}

func GetHexColor(color string) string {
	parts := strings.Split(color, ",")
	if len(parts) != 3 {
		return "FFFFFF"
	}

	r, err1 := strconv.Atoi(strings.TrimSpace(parts[0]))
	g, err2 := strconv.Atoi(strings.TrimSpace(parts[1]))
	b, err3 := strconv.Atoi(strings.TrimSpace(parts[2]))

	if err1 != nil || err2 != nil || err3 != nil {
		return "FFFFFF"
	}

	return fmt.Sprintf("%02X%02X%02X", r, g, b)
}

func GetDisplayName(username string, uuid string) string {
	if emoji := db.EMOJIS[uuid]; emoji != "" {
		return username + " " + emoji
	}
	return username
}

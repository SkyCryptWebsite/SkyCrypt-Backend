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
	"skycrypt/src/security"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/joho/godotenv"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

var colorCodeRegex = regexp.MustCompile("§[0-9a-fk-or]")

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

var domain string
var verboseLogging bool
var forensicsEnabled bool
var armorHexColorsEnabled bool

func init() {
	_ = godotenv.Load()
	domain = os.Getenv("DOMAIN")
	if domain == "" {
		domain = "https://sky.shiiyu.moe"
	}

	verboseLogging = os.Getenv("VERBOSE_LOGGING") == "true"
	forensicsEnabled = os.Getenv("FORENSICS_ENABLED") == "true"
	armorHexColorsEnabled = os.Getenv("ENABLE_ARMOR_HEX") == "true"
}

func GetDomain() string {
	return domain
}

func IsVerboseLogging() bool {
	return verboseLogging
}

func IsForensicsEnabled() bool {
	return forensicsEnabled
}

func IsArmorHexColorsEnabled() bool {
	return armorHexColorsEnabled
}

func LogVerbose(format string, args ...interface{}) {
	if verboseLogging {
		fmt.Printf(format+"\n", args...)
	}
}

func GetRawLore(text string) string {
	return colorCodeRegex.ReplaceAllString(text, "")
}

var nonAsciiRegex = regexp.MustCompile(`[^\x00-\x7F]`)

func RemoveNonAscii(text string) string {
	return nonAsciiRegex.ReplaceAllString(text, "")
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
	if strings.Contains(s, "_") || strings.Contains(s, "-") {
		parts := strings.FieldsFunc(s, func(r rune) bool {
			return r == '_' || r == '-'
		})
		for i, part := range parts {
			parts[i] = cases.Title(language.English).String(part)
		}
		return strings.Join(parts, " ")
	}

	return cases.Title(language.English).String(s)
}

func ParseInt(n string) (int, error) {
	i, err := strconv.Atoi(n)
	if err != nil {
		return 0, err
	}

	return i, nil
}

func RarityNameToInt(rarity string) int {
	for i, r := range constants.RARITIES {
		if strings.EqualFold(r, rarity) {
			return i
		}
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
		fmt.Printf("Unsupported type for FormatNumber: %T\n", v)
		return "0"
	}

	abs := value
	if abs < 0 {
		abs = -abs
	}

	var suffix string
	var divisor float64

	switch {
	case abs >= 1e9:
		suffix = "B"
		divisor = 1e9
	case abs >= 1e6:
		suffix = "M"
		divisor = 1e6
	case abs >= 1e3:
		suffix = "K"
		divisor = 1e3
	default:
		if value == float64(int(value)) {
			return strconv.Itoa(int(value))
		}
		rounded := Round(value, 2)
		return strconv.FormatFloat(rounded, 'f', -1, 64)
	}

	result := value / divisor
	rounded := Round(result, 2)

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
	for i := len(s) - 3; i > 0; i -= 3 {
		s = s[:i] + "," + s[i:]
	}
	return s
}

func ParseTimestamp(timestamp string) int {
	cleanedTimestamp := strings.TrimSpace(strings.ReplaceAll(timestamp, "\u00a0", " "))
	cleanedTimestamp = strings.Join(strings.Fields(cleanedTimestamp), " ")
	layouts := []string{
		"1/2/06 3:04 PM",
		"1/2/06 3:04PM",
		"01/02/06 3:04 PM",
		"01/02/06 3:04PM",
		"1/2/2006 3:04 PM",
		"1/2/2006 3:04PM",
		"01/02/2006 3:04 PM",
		"01/02/2006 3:04PM",
	}

	for _, layout := range layouts {
		t, err := time.Parse(layout, cleanedTimestamp)
		if err == nil {
			return int(t.Unix())
		}
	}

	return 0
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

	var jsonData struct {
		Textures struct {
			SKIN struct {
				URL string `json:"url"`
			} `json:"SKIN"`
		} `json:"textures"`
	}

	if err := json.Unmarshal(data, &jsonData); err != nil {
		return ""
	}

	url := jsonData.Textures.SKIN.URL
	if url == "" {
		return ""
	}

	parts := strings.Split(url, "/")
	if len(parts) == 0 {
		return ""
	}

	return parts[len(parts)-1]
}

func Round(value float64, precision int) float64 {
	if precision < 0 {
		return value
	}
	pow := math.Pow(10, float64(precision))
	return math.Round(value*pow) / pow
}

var replaceVarRegex = regexp.MustCompile(`\{(\w+)\}`)

func ReplaceVariables(template string, variables map[string]float64) string {
	return replaceVarRegex.ReplaceAllStringFunc(template, func(match string) string {
		name := strings.Trim(match, "{}")

		value, exists := variables[name]
		if !exists {
			return match
		}

		// fmt.Printf("Replacing variable %s with value %.2f\n", name, value)
		return fmt.Sprintf("%.0f", value)
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
	} else if a && !b {
		return 1
	}
	return -1
}

func Filter[T any](slice []T, predicate func(T) bool) []T {
	var result []T
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
	if precision < 0 {
		return value
	}
	pow := math.Pow(10, float64(precision))
	return math.Round(value*pow) / pow
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
		fmt.Println("DISCORD_WEBHOOK environment variable not set")
		return
	}

	endpoint = security.RedactString(endpoint)
	errorStr := security.RedactString(fmt.Sprintf("%v", err))
	errorHash := generateErrorHash(endpoint, errorStr)

	if !shouldSendError(errorHash) {
		fmt.Printf("Error webhook rate limited for hash: %s\n", errorHash[:8])
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

	stackStr := security.RedactString(string(stack))
	maxStackLength := 800
	if len(stackStr) > maxStackLength {
		stackStr = stackStr[:maxStackLength] + "\n... (truncated)"
	}

	cleanFilePath := callerInfo
	if strings.Contains(callerInfo, "/") {
		parts := strings.Split(callerInfo, "/")
		if len(parts) >= 2 {
			// Show last 2 directories + file for context
			cleanFilePath = strings.Join(parts[len(parts)-2:], "/")
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
			{
				"name":   "Error Details" + countText,
				"value":  fmt.Sprintf("```\n%s\n```", errorStr),
				"inline": false,
			},
			{
				"name":   "Endpoint",
				"value":  fmt.Sprintf("`%s`", endpoint),
				"inline": true,
			},
			{
				"name":   "Occurred",
				"value":  fmt.Sprintf("<t:%d:R>", time.Now().Unix()),
				"inline": true,
			},
			{
				"name":   "Location",
				"value":  fmt.Sprintf("`%s`", cleanFilePath),
				"inline": false,
			},
			{
				"name":   "Stack Trace",
				"value":  fmt.Sprintf("```go\n%s\n```", stackStr),
				"inline": false,
			},
		},
	}

	payload := map[string]interface{}{
		"username": "SkyCrypt Monitor",
		"embeds":   []map[string]interface{}{embed},
	}

	jsonData, jsonErr := json.Marshal(payload)
	if jsonErr != nil {
		fmt.Printf("Failed to marshal webhook payload: %v\n", security.RedactError(jsonErr))
		return
	}

	resp, httpErr := http.Post(webhookURL, "application/json", bytes.NewBuffer(jsonData))
	if httpErr != nil {
		fmt.Printf("Failed to send webhook: %v\n", security.RedactError(httpErr))
		return
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		fmt.Printf("Webhook returned non-success status: %d\n", resp.StatusCode)
		return
	}
}

func generateErrorHash(endpoint, errorStr string) string {
	data := fmt.Sprintf("%s:%s", endpoint, errorStr)
	hash := md5.Sum([]byte(data))
	return hex.EncodeToString(hash[:])
}

func shouldSendError(errorHash string) bool {
	errorCacheMutex.Lock()
	defer errorCacheMutex.Unlock()

	now := time.Now()
	cache, exists := errorCacheMap[errorHash]

	if !exists {
		errorCacheMap[errorHash] = &errorCache{
			lastSent: now,
			count:    1,
		}
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
	if len(parts) == 3 {
		var r, g, b int
		if _, err := fmt.Sscanf(parts[0], "%d", &r); err != nil {
			return "FFFFFF"
		}
		if _, err := fmt.Sscanf(parts[1], "%d", &g); err != nil {
			return "FFFFFF"
		}
		if _, err := fmt.Sscanf(parts[2], "%d", &b); err != nil {
			return "FFFFFF"
		}
		return fmt.Sprintf("%02X%02X%02X", r, g, b)
	}

	return "FFFFFF"
}

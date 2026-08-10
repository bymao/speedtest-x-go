package main

import (
	"crypto/rand"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	_ "time/tzdata"
)

type config struct {
	WebPort         string
	MaxLogCount     int
	IPService       string
	SameIPMultiLogs bool
	IPInfoAPIKey    string
	DataDir         string
	TimeLocation    *time.Location
}

type app struct {
	cfg   config
	store *store
}

type speedLog struct {
	ID      int     `json:"_id"`
	Key     string  `json:"key,omitempty"`
	IP      string  `json:"ip,omitempty"`
	ISP     string  `json:"isp"`
	Addr    string  `json:"addr"`
	DSpeed  float64 `json:"dspeed"`
	USpeed  float64 `json:"uspeed"`
	Ping    float64 `json:"ping"`
	Jitter  float64 `json:"jitter"`
	Created string  `json:"created"`
}

type storeFile struct {
	NextID int        `json:"next_id"`
	Logs   []speedLog `json:"logs"`
}

type store struct {
	mu   sync.Mutex
	path string
	data storeFile
}

func main() {
	cfg := loadConfig()
	if err := os.MkdirAll(cfg.DataDir, 0o755); err != nil {
		log.Fatalf("create data dir: %v", err)
	}

	st, err := openStore(filepath.Join(cfg.DataDir, "speedlogs.json"))
	if err != nil {
		log.Fatalf("open store: %v", err)
	}

	a := &app{cfg: cfg, store: st}
	mux := http.NewServeMux()
	mux.HandleFunc("/backend/empty.php", a.empty)
	mux.HandleFunc("/backend/garbage.php", a.garbage)
	mux.HandleFunc("/backend/getIP.php", a.getIP)
	mux.HandleFunc("/backend/report.php", a.report)
	mux.HandleFunc("/backend/results-api.php", a.resultsAPI)
	mux.Handle("/", cacheControl(http.FileServer(http.Dir("."))))

	addr := ":" + cfg.WebPort
	log.Printf("speedtest-x-go listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}

func loadConfig() config {
	locName := getenv("TIME_ZONE", "Asia/Shanghai")
	loc, err := time.LoadLocation(locName)
	if err != nil {
		loc = time.FixedZone("Asia/Shanghai", 8*60*60)
	}
	return config{
		WebPort:         getenv("WEBPORT", "80"),
		MaxLogCount:     envInt("MAX_LOG_COUNT", 100),
		IPService:       getenv("IP_SERVICE", "ip.sb"),
		SameIPMultiLogs: envBool("SAME_IP_MULTI_LOGS", false),
		IPInfoAPIKey:    os.Getenv("IPINFO_APIKEY"),
		DataDir:         getenv("SPEEDLOGS_DIR", "/speedlogs"),
		TimeLocation:    loc,
	}
}

func getenv(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) int {
	v, err := strconv.Atoi(strings.TrimSpace(os.Getenv(key)))
	if err != nil || v <= 0 {
		return fallback
	}
	return v
}

func envBool(key string, fallback bool) bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
	if v == "" {
		return fallback
	}
	return v == "1" || v == "true" || v == "yes" || v == "on"
}

func openStore(path string) (*store, error) {
	s := &store{path: path, data: storeFile{NextID: 1}}
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return nil, err
	}
	if len(strings.TrimSpace(string(b))) == 0 {
		return s, nil
	}
	if err := json.Unmarshal(b, &s.data); err != nil {
		var logs []speedLog
		if err2 := json.Unmarshal(b, &logs); err2 != nil {
			return nil, err
		}
		s.data.Logs = logs
	}
	maxID := 0
	for _, l := range s.data.Logs {
		if l.ID > maxID {
			maxID = l.ID
		}
	}
	if s.data.NextID <= maxID {
		s.data.NextID = maxID + 1
	}
	if s.data.NextID <= 0 {
		s.data.NextID = 1
	}
	return s, nil
}

func (s *store) saveLocked() error {
	tmp := s.path + ".tmp"
	b, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

func (s *store) upsert(logEntry speedLog, sameIPMultiLogs bool, maxCount int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	match := -1
	for i := range s.data.Logs {
		if sameIPMultiLogs {
			if s.data.Logs[i].Key == logEntry.Key {
				match = i
			}
		} else if s.data.Logs[i].IP == logEntry.IP {
			match = i
		}
	}

	if match >= 0 {
		logEntry.ID = s.data.Logs[match].ID
		if sameIPMultiLogs {
			logEntry.Key = s.data.Logs[match].Key
		} else {
			logEntry.IP = s.data.Logs[match].IP
		}
		s.data.Logs[match] = logEntry
	} else {
		logEntry.ID = s.data.NextID
		s.data.NextID++
		s.data.Logs = append(s.data.Logs, logEntry)
	}

	sort.SliceStable(s.data.Logs, func(i, j int) bool {
		return s.data.Logs[i].ID > s.data.Logs[j].ID
	})
	if maxCount > 0 && len(s.data.Logs) > maxCount {
		s.data.Logs = s.data.Logs[:maxCount]
	}
	sort.SliceStable(s.data.Logs, func(i, j int) bool {
		return s.data.Logs[i].ID < s.data.Logs[j].ID
	})
	return s.saveLocked()
}

func (s *store) latest(limit int) []speedLog {
	s.mu.Lock()
	defer s.mu.Unlock()

	logs := append([]speedLog(nil), s.data.Logs...)
	if logs == nil {
		return []speedLog{}
	}
	sort.SliceStable(logs, func(i, j int) bool {
		return logs[i].Created > logs[j].Created
	})
	if limit > 0 && len(logs) > limit {
		logs = logs[:limit]
	}
	return logs
}

func (a *app) empty(w http.ResponseWriter, r *http.Request) {
	applyCommonHeaders(w, r)
	w.WriteHeader(http.StatusOK)
}

func (a *app) garbage(w http.ResponseWriter, r *http.Request) {
	applyCommonHeaders(w, r)
	w.Header().Set("Content-Description", "File Transfer")
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", "attachment; filename=random.dat")
	w.Header().Set("Content-Transfer-Encoding", "binary")

	chunks := 4
	if ck := r.URL.Query().Get("ckSize"); regexp.MustCompile(`^\d+$`).MatchString(ck) {
		if n, err := strconv.Atoi(ck); err == nil && n > 0 {
			chunks = n
			if chunks > 50 {
				chunks = 50
			}
		}
	}

	buf := make([]byte, 1024*1024)
	if _, err := rand.Read(buf); err != nil {
		http.Error(w, "random data unavailable", http.StatusInternalServerError)
		return
	}
	for i := 0; i < chunks; i++ {
		if _, err := w.Write(buf); err != nil {
			return
		}
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
	}
}

func (a *app) getIP(w http.ResponseWriter, r *http.Request) {
	applyCommonHeaders(w, r)
	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	ip := clientIP(r)
	if local := localOrPrivateIPInfo(ip); local != "" {
		writeIPResponse(w, ip, local, nil)
		return
	}
	if _, ok := r.URL.Query()["isp"]; !ok {
		writeIPResponse(w, ip, "", nil)
		return
	}

	raw := a.fetchISPInfo(ip)
	isp := ispName(raw, a.cfg.IPService)
	writeIPResponse(w, ip, isp, raw)
}

func (a *app) report(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseForm(); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	maskedIP := maskLastSegment(r.FormValue("ip"))
	if maskedIP == "" {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	sum := sha1.Sum([]byte(r.FormValue("key")))
	entry := speedLog{
		Key:     hex.EncodeToString(sum[:]),
		IP:      maskedIP,
		ISP:     sanitize(r.FormValue("isp")),
		Addr:    sanitize(r.FormValue("addr")),
		DSpeed:  parseFloat(r.FormValue("dspeed")),
		USpeed:  parseFloat(r.FormValue("uspeed")),
		Ping:    parseFloat(r.FormValue("ping")),
		Jitter:  parseFloat(r.FormValue("jitter")),
		Created: time.Now().In(a.cfg.TimeLocation).Format("2006-01-02 15:04:05"),
	}
	if err := a.store.upsert(entry, a.cfg.SameIPMultiLogs, a.cfg.MaxLogCount); err != nil {
		log.Printf("save report: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *app) resultsAPI(w http.ResponseWriter, r *http.Request) {
	applyCommonHeaders(w, r)
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"code": 0,
		"data": a.store.latest(a.cfg.MaxLogCount),
	})
}

func clientIP(r *http.Request) string {
	for _, h := range []string{"Client-IP", "X-Real-IP", "X-Forwarded-For"} {
		v := strings.TrimSpace(r.Header.Get(h))
		if v == "" {
			continue
		}
		if h == "X-Forwarded-For" {
			v = strings.TrimSpace(strings.Split(v, ",")[0])
		}
		return strings.TrimPrefix(v, "::ffff:")
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	return strings.TrimPrefix(host, "::ffff:")
}

func localOrPrivateIPInfo(ip string) string {
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return ""
	}
	if parsed.IsLoopback() {
		if parsed.To4() != nil {
			return "localhost IPv4 access"
		}
		return "localhost IPv6 access"
	}
	if parsed.IsPrivate() {
		return "private IPv4 access"
	}
	if parsed.IsLinkLocalUnicast() {
		if parsed.To4() != nil {
			return "link-local IPv4 access"
		}
		return "link-local IPv6 access"
	}
	return ""
}

func (a *app) fetchISPInfo(ip string) map[string]any {
	var url string
	switch a.cfg.IPService {
	case "ipinfo.io":
		url = "https://ipinfo.io/" + ip + "/json"
		if a.cfg.IPInfoAPIKey != "" {
			url += "?token=" + a.cfg.IPInfoAPIKey
		}
	default:
		url = "https://api.ip.sb/geoip/" + ip
	}

	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil
	}
	var data map[string]any
	if err := json.Unmarshal(body, &data); err != nil {
		return nil
	}
	return data
}

func ispName(raw map[string]any, service string) string {
	if raw == nil {
		return "Unknown"
	}
	if service == "ipinfo.io" {
		if org, ok := raw["org"].(string); ok && org != "" {
			return regexp.MustCompile(`^AS\d+\s+`).ReplaceAllString(org, "")
		}
		return "Unknown"
	}
	if org, ok := raw["organization"].(string); ok && org != "" {
		return org
	}
	return "Unknown"
}

func writeIPResponse(w http.ResponseWriter, ip, ipInfo string, raw map[string]any) {
	processed := ip
	if ipInfo != "" {
		processed += " - " + ipInfo
	}
	if raw != nil {
		country, cok := raw["country"].(string)
		region, rok := raw["region"].(string)
		city, ciok := raw["city"].(string)
		if cok && rok && ciok {
			processed += " - " + country + "," + region + "," + city
		}
	}

	rawValue := any("")
	if raw != nil {
		rawValue = raw
	}
	_ = json.NewEncoder(w).Encode(map[string]any{
		"processedString": processed,
		"rawIspInfo":      rawValue,
	})
}

func maskLastSegment(ip string) string {
	parsed := net.ParseIP(strings.TrimSpace(ip))
	if parsed == nil {
		return ""
	}
	if v4 := parsed.To4(); v4 != nil {
		return fmt.Sprintf("%d.%d.%d.*", v4[0], v4[1], v4[2])
	}
	v16 := parsed.To16()
	if v16 == nil {
		return ""
	}
	v16[14], v16[15] = 0, 0
	return strings.TrimRight(net.IP(v16).String(), "0") + "*"
}

func sanitize(v string) string {
	v = strings.TrimSpace(v)
	v = strings.Map(func(r rune) rune {
		if r == '\x00' {
			return -1
		}
		return r
	}, v)
	if len(v) > 512 {
		return v[:512]
	}
	return v
}

func parseFloat(v string) float64 {
	f, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
	if err != nil || math.IsNaN(f) || math.IsInf(f, 0) {
		return 0
	}
	return f
}

func applyCommonHeaders(w http.ResponseWriter, r *http.Request) {
	if _, ok := r.URL.Query()["cors"]; ok {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Encoding, Content-Type")
	}
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0, s-maxage=0")
	w.Header().Set("Pragma", "no-cache")
}

func cacheControl(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r)
	})
}

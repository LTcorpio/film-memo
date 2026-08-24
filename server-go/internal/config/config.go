// Package config 负责加载环境变量与项目配置。
// 对应 JS 版的 dotenv.config 与各 process.env 读取逻辑。
package config

import (
	"bufio"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Config 汇总后端运行所需的全部配置项。
type Config struct {
	TmdbAccessToken string // TMDB v4 Bearer Token（推荐）
	TmdbAPIKey      string // TMDB v3 API Key
	Port            string // HTTP 服务端口
	ExcelPath       string // Excel 数据源路径
	DBPath          string // SQLite 数据库文件路径
	ImagesDir       string // 本地图片存储目录
	ProjectRoot     string // 项目根目录（用于定位 client/dist 等）
}

// FindProjectRoot 自下而上查找项目根目录。
// 判据：目录下存在 client/ 或 .env 或 server-go/ 任一标记。
// 开发时在 server-go/ 下运行 go run，向上即可命中 film-memo/；
// Docker 生产环境 WORKDIR=/app，/app 下含 client/dist、.env，直接命中。
// 若环境变量 FILM_MEMO_ROOT 已设置，则直接采用。
func FindProjectRoot() string {
	if root := os.Getenv("FILM_MEMO_ROOT"); root != "" {
		return root
	}
	candidates := []string{}
	if exe, err := os.Executable(); err == nil {
		candidates = append(candidates, filepath.Dir(exe))
	}
	if cwd, err := os.Getwd(); err == nil {
		candidates = append(candidates, cwd)
	}
	for _, start := range candidates {
		dir := start
		for i := 0; i < 12; i++ {
			if isProjectRoot(dir) {
				return dir
			}
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
			dir = parent
		}
	}
	// 兜底：当前工作目录
	if cwd, err := os.Getwd(); err == nil {
		return cwd
	}
	return "."
}

func isProjectRoot(dir string) bool {
	// 目录标记：client / server-go / server
	for _, d := range []string{"client", "server-go", "server"} {
		info, err := os.Stat(filepath.Join(dir, d))
		if err != nil || !info.IsDir() {
			continue
		}
		if d == "server" {
			// server-go 目录本身也含 server/ 子目录，但带 go.mod，需排除
			if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
				continue
			}
		}
		return true
	}
	// 文件标记：.env
	if _, err := os.Stat(filepath.Join(dir, ".env")); err == nil {
		return true
	}
	return false
}

// LoadDotEnv 解析 .env 文件并注入 os.environ，已存在的环境变量不被覆盖（与 dotenv 默认行为一致）。
func LoadDotEnv(path string) {
	f, err := os.Open(path)
	if err != nil {
		return // .env 不存在时静默忽略
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// 去除可选的 export 前缀
		if strings.HasPrefix(line, "export ") {
			line = strings.TrimSpace(strings.TrimPrefix(line, "export"))
		}
		eq := strings.IndexByte(line, '=')
		if eq <= 0 {
			continue
		}
		key := strings.TrimSpace(line[:eq])
		val := line[eq+1:]
		// 去除首尾成对引号
		if len(val) >= 2 {
			if (val[0] == '"' && val[len(val)-1] == '"') || (val[0] == '\'' && val[len(val)-1] == '\'') {
				val = val[1 : len(val)-1]
			}
		}
		// 不覆盖已存在的环境变量
		if _, ok := os.LookupEnv(key); !ok {
			_ = os.Setenv(key, val)
		}
	}
}

// Load 读取配置：先定位项目根 → 加载 .env → 读取环境变量并应用默认值。
func Load() Config {
	root := FindProjectRoot()
	LoadDotEnv(filepath.Join(root, ".env"))

	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = filepath.Join(root, "data", "films.db")
	}
	imagesDir := os.Getenv("IMAGES_DIR")
	if imagesDir == "" {
		imagesDir = filepath.Join(root, "data", "images")
	}
	excelPath := os.Getenv("EXCEL_PATH")
	if excelPath == "" {
		if home, err := os.UserHomeDir(); err == nil {
			excelPath = filepath.Join(home, "Desktop", "影视观看记录.xlsx")
		}
	}
	port := os.Getenv("PORT")
	if port == "" {
		port = "4000"
	}

	return Config{
		TmdbAccessToken: os.Getenv("TMDB_ACCESS_TOKEN"),
		TmdbAPIKey:      os.Getenv("TMDB_API_KEY"),
		Port:            port,
		ExcelPath:       excelPath,
		DBPath:          dbPath,
		ImagesDir:       imagesDir,
		ProjectRoot:     root,
	}
}

// TmdbConfigured 是否配置了任一 TMDB 凭证。
func (c Config) TmdbConfigured() bool {
	return c.TmdbAccessToken != "" || c.TmdbAPIKey != ""
}

// PortInt 以 int 返回端口，解析失败回退 4000。
func (c Config) PortInt() int {
	if n, err := strconv.Atoi(c.Port); err == nil && n > 0 {
		return n
	}
	return 4000
}

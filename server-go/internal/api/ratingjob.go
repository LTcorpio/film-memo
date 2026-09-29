package api

import (
	"context"
	"sync"
	"time"

	"film-memo/internal/db"
)

// --- 评分刷新后台任务 ---
//
// 豆瓣逐条限速 1.2s，200 多条影片需数分钟，故刷新改为后台任务：
// POST /api/ratings/refresh 立即返回，实际抓取在 goroutine 中进行，
// 前端轮询 GET /api/ratings/refresh/progress 获取进度。
// 同一时刻只允许一个任务，重复提交不会新建（返回 started=false）。

// 任务阶段（供前端映射为进度文案）。
const (
	phasePrep   = "prep"   // 读取待刷新的影片清单
	phaseImdb   = "imdb"   // 下载/校验 IMDb 数据集并批量查询
	phaseDouban = "douban" // 逐部抓取豆瓣评分
	phaseWrite  = "write"  // 写入数据库与同步记录
	phaseDone   = "done"   // 完成
)

// ratingJob 是一次评分刷新的运行状态，所有字段由 mu 保护。
type ratingJob struct {
	mu         sync.Mutex
	running    bool
	stopped    bool   // 是否被用户手动停止（区别于自然跑完）
	source     string // "" 表示两个来源都跑
	phase      string
	done       int64
	total      int64
	startedAt  string
	finishedAt *string
	stats      []*sourceStat // 各来源累计统计（复用既有结构）
	result     *refreshSummary
	errMsg     *string
	cancel     context.CancelFunc // 停机时取消任务
}

// jobProgress 是 GET /api/ratings/refresh/progress 的响应体。
type jobProgress struct {
	Running    bool            `json:"running"`
	Stopped    bool            `json:"stopped"`
	Source     string          `json:"source"`
	Phase      string          `json:"phase"`
	Done       int64           `json:"done"`
	Total      int64           `json:"total"`
	StartedAt  *string         `json:"startedAt"`
	FinishedAt *string         `json:"finishedAt"`
	Result     *refreshSummary `json:"result"`
	Error      *string         `json:"error"`
}

// snapshot 返回当前进度快照（深拷贝统计，避免与后台 goroutine 竞争）。
func (j *ratingJob) snapshot() jobProgress {
	j.mu.Lock()
	defer j.mu.Unlock()
	p := jobProgress{
		Running: j.running,
		Stopped: j.stopped,
		Source:  j.source,
		Phase:   j.phase,
		Done:    j.done,
		Total:   j.total,
	}
	if started := j.startedAt; started != "" {
		p.StartedAt = &started
	}
	p.FinishedAt = j.finishedAt
	p.Result = j.result
	p.Error = j.errMsg
	return p
}

// setPhase 更新阶段（同时可推进 done/total）。
func (j *ratingJob) setPhase(phase string) {
	j.mu.Lock()
	j.phase = phase
	j.mu.Unlock()
}

// setTotal 设定本次任务的总进度单位。
func (j *ratingJob) setTotal(total int64) {
	j.mu.Lock()
	j.total = total
	j.mu.Unlock()
}

// advance 推进一个进度单位。
func (j *ratingJob) advance() {
	j.mu.Lock()
	j.done++
	j.mu.Unlock()
}

// finish 标记任务结束，落定结果与结束时间。被手动停止时保留实际完成数，不伪装成跑满。
func (j *ratingJob) finish(result *refreshSummary, stats []*sourceStat, errMsg *string, finishedAt string) {
	j.mu.Lock()
	j.running = false
	j.phase = phaseDone
	if !j.stopped {
		j.done = j.total
	}
	j.finishedAt = &finishedAt
	j.result = result
	j.stats = stats
	j.errMsg = errMsg
	j.mu.Unlock()
}

// jobRegistry 保存当前/最近一次的评分刷新任务。
type jobRegistry struct {
	mu  sync.Mutex
	cur *ratingJob
}

// start 尝试创建新任务。已有任务运行中时返回该任务并返回 false（不新建）。
func (r *jobRegistry) start(source string, startedAt string) (*ratingJob, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.cur != nil && r.cur.isRunning() {
		return r.cur, false
	}
	j := &ratingJob{running: true, source: source, phase: phasePrep, startedAt: startedAt}
	r.cur = j
	return j, true
}

// current 返回当前/最近一次任务，可能为 nil。
func (r *jobRegistry) current() *ratingJob {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.cur
}

// isRunning 读取任务的运行中标志。
func (j *ratingJob) isRunning() bool {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.running
}

// bindCancel 记录任务的可取消句柄（停机时使用）。
func (j *ratingJob) bindCancel(cancel context.CancelFunc) {
	j.mu.Lock()
	j.cancel = cancel
	j.mu.Unlock()
}

// cancelIfRunning 取消进行中的任务（用户点「停止」或服务停机）；返回是否进行了取消。
func (j *ratingJob) cancelIfRunning() bool {
	j.mu.Lock()
	cancel := j.cancel
	running := j.running
	if running && cancel != nil {
		j.stopped = true
	}
	j.mu.Unlock()
	if running && cancel != nil {
		cancel()
		return true
	}
	return false
}

// wait 等待任务结束，最多等待 timeout（停机时用，确保后台任务不再写库）。
func (j *ratingJob) wait(timeout time.Duration) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if !j.isRunning() {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// progressUnits 计算本次任务的进度单位总数：
// 豆瓣逐条抓取按影片计数；IMDb 是一次性批量操作，整体计为 1 个单位。
// 只统计本次真正会去抓的来源（有对应 ID 且判定为缺失）。
func progressUnits(rows []db.FilmRef, runDouban, runImdb bool) int64 {
	var n int64
	if runDouban {
		for i := range rows {
			if rows[i].DoubanID != "" && rows[i].DoubanNeed {
				n++
			}
		}
	}
	if runImdb {
		for i := range rows {
			if rows[i].ImdbID != "" && rows[i].ImdbNeed {
				n++
				break
			}
		}
	}
	return n
}

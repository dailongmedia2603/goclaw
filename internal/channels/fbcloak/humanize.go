//go:build !sqliteonly

package fbcloak

import (
	"math"
	"math/rand"
	"time"
)

// Humanizer produces randomized timing + path values that approximate human
// input rhythms. The Rng source is exported (via NewHumanizer seed) so tests
// can reproduce sequences deterministically.
type Humanizer struct {
	rng *rand.Rand
	cfg HumanizeConfig
}

// HumanizeConfig is the knob set; defaults match research §6 (cloak browser
// research doc) — VN office hours, 80–180 ms per char, 30–180 s pre-send delay.
type HumanizeConfig struct {
	TypingMinMS, TypingMaxMS         int
	PreActionMin, PreActionMax       time.Duration
	BetweenSendMin, BetweenSendMax   time.Duration
	SpacePauseExtraMS, DotPauseExtra int
	HesitationProbability            float64 // 0..1; e.g. 0.05 = 5 %
	HesitationMS                     int
	WorkingHours                     WorkingHours
	BezierJitterPx                   float64
	BezierSteps                      int
}

// DefaultHumanizeConfig returns the production defaults. Test code may
// override individual fields after construction.
func DefaultHumanizeConfig() HumanizeConfig {
	return HumanizeConfig{
		TypingMinMS:           80,
		TypingMaxMS:           180,
		PreActionMin:          30 * time.Second,
		PreActionMax:          180 * time.Second,
		BetweenSendMin:        60 * time.Second,
		BetweenSendMax:        300 * time.Second,
		SpacePauseExtraMS:     50,
		DotPauseExtra:         200,
		HesitationProbability: 0.05,
		HesitationMS:          800,
		BezierJitterPx:        2.5,
		BezierSteps:           45,
		WorkingHours:          WorkingHours{Start: "08:00", End: "21:00", TZ: "Asia/Ho_Chi_Minh"},
	}
}

// NewHumanizer constructs a humanizer with a seed (zero seed → deterministic).
// Pass DefaultHumanizeConfig() explicitly for production defaults; this
// constructor does NOT auto-fill missing fields so test code retains full
// control over the knobs it cares about.
func NewHumanizer(seed int64, cfg HumanizeConfig) *Humanizer {
	return &Humanizer{
		rng: rand.New(rand.NewSource(seed)),
		cfg: cfg,
	}
}

// TypingDelay returns the per-character delay for the given rune. Rules:
//   - base uniform in [TypingMinMS, TypingMaxMS]
//   - +SpacePauseExtraMS after a space
//   - +DotPauseExtra after a sentence terminator (.!?)
//   - small (HesitationProbability) chance of a long hesitation pause
func (h *Humanizer) TypingDelay(prev rune) time.Duration {
	span := max(h.cfg.TypingMaxMS-h.cfg.TypingMinMS, 0)
	d := time.Duration(h.cfg.TypingMinMS+h.rng.Intn(span+1)) * time.Millisecond
	switch prev {
	case ' ', '\n', '\t':
		d += time.Duration(h.cfg.SpacePauseExtraMS) * time.Millisecond
	case '.', '!', '?':
		d += time.Duration(h.cfg.DotPauseExtra) * time.Millisecond
	}
	if h.rng.Float64() < h.cfg.HesitationProbability {
		d += time.Duration(h.cfg.HesitationMS) * time.Millisecond
	}
	return d
}

// PreActionDelay is the random pause before opening / clicking on something.
func (h *Humanizer) PreActionDelay() time.Duration {
	return randomDuration(h.rng, h.cfg.PreActionMin, h.cfg.PreActionMax)
}

// BetweenSendDelay is the random gap between two send operations on the
// same fanpage in one job run.
func (h *Humanizer) BetweenSendDelay() time.Duration {
	return randomDuration(h.rng, h.cfg.BetweenSendMin, h.cfg.BetweenSendMax)
}

// IsWithinWorkingHours returns whether t (in cfg.WorkingHours.TZ) is
// inside the configured Start..End window. Returns true if the WorkingHours
// is the zero value (i.e. always-on).
func (h *Humanizer) IsWithinWorkingHours(t time.Time) bool {
	wh := h.cfg.WorkingHours
	if wh.Start == "" || wh.End == "" {
		return true
	}
	loc, err := time.LoadLocation(wh.TZ)
	if err != nil {
		loc = time.UTC
	}
	local := t.In(loc)
	hh, mm := local.Hour(), local.Minute()
	startH, startM, ok1 := parseHHMM(wh.Start)
	endH, endM, ok2 := parseHHMM(wh.End)
	if !ok1 || !ok2 {
		return true
	}
	cur := hh*60 + mm
	startTotal := startH*60 + startM
	endTotal := endH*60 + endM
	if endTotal < startTotal {
		// Window wraps midnight — include early-morning hours.
		return cur >= startTotal || cur <= endTotal
	}
	return cur >= startTotal && cur <= endTotal
}

// BezierPath returns a sequence of (x,y) points approximating a human cursor
// path between (fromX, fromY) and (toX, toY). Two random control points are
// chosen ±jitter around the midpoint, producing organic cubic bézier curves.
// Each step has a small per-axis jitter to avoid pixel-perfect frames.
func (h *Humanizer) BezierPath(fromX, fromY, toX, toY int) [][2]int {
	steps := h.cfg.BezierSteps
	if steps <= 0 {
		steps = 30
	}
	jitter := h.cfg.BezierJitterPx
	mx := float64(fromX+toX) / 2
	my := float64(fromY+toY) / 2
	// Two control points perturbed around the midpoint.
	c1x := mx + (h.rng.Float64()*2-1)*jitter*4
	c1y := my + (h.rng.Float64()*2-1)*jitter*4
	c2x := mx + (h.rng.Float64()*2-1)*jitter*4
	c2y := my + (h.rng.Float64()*2-1)*jitter*4

	points := make([][2]int, 0, steps+1)
	for i := 0; i <= steps; i++ {
		t := float64(i) / float64(steps)
		x, y := cubicBezier(float64(fromX), float64(fromY), c1x, c1y, c2x, c2y, float64(toX), float64(toY), t)
		// Per-step jitter.
		x += (h.rng.Float64()*2 - 1) * jitter
		y += (h.rng.Float64()*2 - 1) * jitter
		points = append(points, [2]int{int(math.Round(x)), int(math.Round(y))})
	}
	return points
}

func cubicBezier(p0x, p0y, p1x, p1y, p2x, p2y, p3x, p3y, t float64) (float64, float64) {
	u := 1 - t
	tt := t * t
	uu := u * u
	uuu := uu * u
	ttt := tt * t
	x := uuu*p0x + 3*uu*t*p1x + 3*u*tt*p2x + ttt*p3x
	y := uuu*p0y + 3*uu*t*p1y + 3*u*tt*p2y + ttt*p3y
	return x, y
}

func randomDuration(rng *rand.Rand, min, max time.Duration) time.Duration {
	if max <= min {
		return min
	}
	delta := rng.Int63n(int64(max - min))
	return min + time.Duration(delta)
}

func parseHHMM(s string) (int, int, bool) {
	if len(s) < 4 {
		return 0, 0, false
	}
	var h, m int
	for _, sep := range []string{":"} {
		if i := indexOf(s, sep); i > 0 {
			if v, ok := atoi(s[:i]); ok {
				h = v
			} else {
				return 0, 0, false
			}
			if v, ok := atoi(s[i+1:]); ok {
				m = v
			} else {
				return 0, 0, false
			}
			break
		}
	}
	if h < 0 || h > 23 || m < 0 || m > 59 {
		return 0, 0, false
	}
	return h, m, true
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

func atoi(s string) (int, bool) {
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, false
		}
		n = n*10 + int(c-'0')
	}
	return n, true
}

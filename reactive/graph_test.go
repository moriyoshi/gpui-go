package reactive_test

import (
	"errors"
	"reflect"
	"testing"

	"github.com/moriyoshi/gpui-go/reactive"
)

func check(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
func TestDynamicDependenciesAndMemoEquality(t *testing.T) {
	g := reactive.NewGraph()
	defer g.Dispose()
	owner := g.Root()
	active := reactive.NewSignal(owner, true)
	source := reactive.NewSignal(owner, 1)
	other := reactive.NewSignal(owner, 10)
	memoRuns, effectRuns := 0, 0
	memo := reactive.NewMemo(owner, func() int {
		memoRuns++
		if active.Get() {
			return source.Get() % 2
		}
		return other.Get()
	})
	_, err := reactive.NewEffect(owner, func(*reactive.Scope) error { _ = memo.Get(); effectRuns++; return nil })
	check(t, err)
	check(t, source.Set(3))
	if memoRuns != 2 || effectRuns != 1 {
		t.Fatalf("equal memo propagated: memo=%d effect=%d", memoRuns, effectRuns)
	}
	check(t, active.Set(false))
	check(t, source.Set(4))
	if memoRuns != 3 || effectRuns != 2 {
		t.Fatalf("stale branch retained dependency: %d %d", memoRuns, effectRuns)
	}
	check(t, other.Set(11))
	if effectRuns != 3 {
		t.Fatal("active branch did not notify")
	}
}
func TestDiamondIsGlitchFreeAndBatchesPullFreshMemos(t *testing.T) {
	g := reactive.NewGraph()
	defer g.Dispose()
	s := reactive.NewSignal(g.Root(), 1)
	a := reactive.NewMemo(g.Root(), func() int { return s.Get() * 2 })
	b := reactive.NewMemo(g.Root(), func() int { return s.Get() * 3 })
	var seen []int
	_, err := reactive.NewEffect(g.Root(), func(*reactive.Scope) error { seen = append(seen, a.Get()+b.Get()); return nil })
	check(t, err)
	check(t, g.Batch(func() error {
		check(t, s.Set(2))
		if a.Get() != 4 {
			t.Fatal("memo read inside batch was stale")
		}
		return s.Set(3)
	}))
	if !reflect.DeepEqual(seen, []int{5, 15}) {
		t.Fatalf("diamond exposed intermediate values: %v", seen)
	}
}
func TestOwnerCleanupNestedEffectsAndUntrackedReads(t *testing.T) {
	g := reactive.NewGraph()
	defer g.Dispose()
	s := reactive.NewSignal(g.Root(), 0)
	untracked := reactive.NewSignal(g.Root(), 0)
	owner := g.Root().Child()
	runs, childRuns, cleanups := 0, 0, 0
	_, err := reactive.NewEffect(owner, func(run *reactive.Scope) error {
		_ = s.Get()
		g.Untrack(func() { _ = untracked.Get() })
		// A lifetime ended during evaluation must not subscribe its parent
		// through incidental reads in cleanup callbacks.
		temporary := run.Child()
		temporary.OnCleanup(func() { _ = untracked.Get() })
		temporary.Dispose()
		runs++
		run.OnCleanup(func() { cleanups++ })
		_, err := reactive.NewEffect(run, func(*reactive.Scope) error { _ = untracked.Get(); childRuns++; return nil })
		return err
	})
	check(t, err)
	check(t, untracked.Set(1))
	if runs != 1 || childRuns != 2 {
		t.Fatalf("untrack/nesting: %d %d", runs, childRuns)
	}
	check(t, s.Set(1))
	if cleanups != 1 || g.Stats().Computations != 2 {
		t.Fatalf("old child leaked: %d %+v", cleanups, g.Stats())
	}
	owner.Dispose()
	check(t, s.Set(2))
	check(t, untracked.Set(2))
	if runs != 2 || cleanups != 2 || g.Stats().Computations != 0 {
		t.Fatalf("disposed owner still observed: %d %d %+v", runs, cleanups, g.Stats())
	}
}
func TestFailedComputationsNeverPublishAndCanRecover(t *testing.T) {
	g := reactive.NewGraph()
	defer g.Dispose()
	bad := reactive.NewSignal(g.Root(), false)
	failure := errors.New("calculation failed")
	published := 0
	g.OnSettled(func() error { published++; return nil })
	_, err := reactive.NewEffect(g.Root(), func(*reactive.Scope) error {
		if bad.Get() {
			return failure
		}
		return nil
	})
	check(t, err)
	published = 0
	if err := bad.Set(true); !errors.Is(err, failure) {
		t.Fatalf("lost computation failure: %v", err)
	}
	if err := g.Flush(); !errors.Is(err, failure) || published != 0 {
		t.Fatalf("failed frame published: %v %d", err, published)
	}
	check(t, bad.Set(false))
	if published != 1 {
		t.Fatalf("recovery did not publish: %d", published)
	}
}
func TestDisposingFailedComputationUnblocksFrames(t *testing.T) {
	g := reactive.NewGraph()
	defer g.Dispose()
	published := 0
	g.OnSettled(func() error { published++; return nil })
	failure := errors.New("calculation failed")
	effect, err := reactive.NewEffect(g.Root(), func(*reactive.Scope) error { return failure })
	if !errors.Is(err, failure) {
		t.Fatalf("lost computation failure: %v", err)
	}
	if err := g.Flush(); !errors.Is(err, failure) || published != 0 {
		t.Fatalf("failed frame published: %v %d", err, published)
	}
	effect.Dispose()
	if err := g.Flush(); err != nil || published != 1 {
		t.Fatalf("disposed failure still blocks frames: %v %d", err, published)
	}
}
func TestComputationDisposedDuringFailingRunDoesNotBlockFrames(t *testing.T) {
	g := reactive.NewGraph()
	defer g.Dispose()
	published := 0
	g.OnSettled(func() error { published++; return nil })
	trigger := reactive.NewSignal(g.Root(), false)
	failure := errors.New("calculation failed")
	var effect *reactive.Effect
	var err error
	effect, err = reactive.NewEffect(g.Root(), func(*reactive.Scope) error {
		if trigger.Get() {
			effect.Dispose()
			return failure
		}
		return nil
	})
	check(t, err)
	published = 0
	if err := trigger.Set(true); !errors.Is(err, failure) {
		t.Fatalf("lost computation failure: %v", err)
	}
	if err := g.Flush(); err != nil || published != 1 {
		t.Fatalf("disposed failure still blocks frames: %v %d", err, published)
	}
}
func TestRunawayLimitIsPerFlush(t *testing.T) {
	g := reactive.NewGraph()
	defer g.Dispose()
	count := reactive.NewSignal(g.Root(), 0)
	_, err := reactive.NewEffect(g.Root(), func(*reactive.Scope) error {
		if value := count.Get(); value%600 != 0 {
			return count.Set(value + 1)
		}
		return nil
	})
	check(t, err)
	check(t, count.Set(1))
	check(t, count.Set(601))
	if got := count.Peek(); got != 1200 {
		t.Fatalf("effect stopped early: %d", got)
	}
}
func TestMemoCycleAndEffectRunawayAreBounded(t *testing.T) {
	g := reactive.NewGraph()
	defer g.Dispose()
	cycle := reactive.NewSignal(g.Root(), false)
	var memo *reactive.Memo[int]
	memo = reactive.NewMemo(g.Root(), func() int {
		if cycle.Get() {
			return memo.Get()
		}
		return 1
	})
	_, err := reactive.NewEffect(g.Root(), func(*reactive.Scope) error { _ = memo.Get(); return nil })
	check(t, err)
	if err := cycle.Set(true); !errors.Is(err, reactive.ErrCycle) {
		t.Fatalf("cycle was not detected: %v", err)
	}
	check(t, cycle.Set(false))
	if memo.Get() != 1 {
		t.Fatal("cycle recovery failed")
	}
	h := reactive.NewGraph()
	defer h.Dispose()
	s := reactive.NewSignal(h.Root(), 0)
	loop := reactive.NewSignal(h.Root(), true)
	published := 0
	h.OnSettled(func() error { published++; return nil })
	_, err = reactive.NewEffect(h.Root(), func(*reactive.Scope) error {
		if loop.Get() {
			return s.Set(s.Get() + 1)
		}
		return nil
	})
	if !errors.Is(err, reactive.ErrRunaway) {
		t.Fatalf("runaway was not bounded: %v", err)
	}
	if err := h.Flush(); !errors.Is(err, reactive.ErrRunaway) || published != 0 {
		t.Fatalf("runaway published a partial result: %v %d", err, published)
	}
	check(t, loop.Set(false))
	if published != 1 {
		t.Fatal("runaway did not recover after an input change")
	}
}

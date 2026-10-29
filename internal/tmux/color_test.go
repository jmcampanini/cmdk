package tmux

import (
	"context"
	"testing"
	"time"
)

func TestSupportsTrueColor(t *testing.T) {
	for _, test := range []struct {
		name   string
		output string
		want   bool
	}{
		{name: "RGB enabled", output: "192: RGB: (flag) true\n", want: true},
		{name: "Tc enabled", output: "193: Tc: (flag) true\n", want: true},
		{name: "disabled", output: "192: RGB: (flag) false\n193: Tc: (flag) false\n"},
		{name: "unrelated flag", output: "0: AX: (flag) true\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			writeFakeTmux(t, `[ "$#" -eq 1 ] && [ "$1" = info ] || exit 1
printf '%s' "$CMDK_TEST_TMUX_INFO"`)
			t.Setenv("CMDK_TEST_TMUX_INFO", test.output)

			if got := SupportsTrueColor(context.Background(), time.Second); got != test.want {
				t.Errorf("SupportsTrueColor() = %t, want %t", got, test.want)
			}
		})
	}
}

func TestSupportsTrueColorBoundsFailedQueries(t *testing.T) {
	for _, test := range []struct {
		name string
		body string
	}{
		{name: "failed command", body: "exit 1"},
		{name: "timeout", body: "exec /bin/sleep 5"},
	} {
		t.Run(test.name, func(t *testing.T) {
			writeFakeTmux(t, test.body)
			start := time.Now()

			if SupportsTrueColor(context.Background(), 20*time.Millisecond) {
				t.Error("SupportsTrueColor() = true after a failed query")
			}
			if elapsed := time.Since(start); elapsed > time.Second {
				t.Errorf("color query took %s, want less than 1s", elapsed)
			}
		})
	}
}

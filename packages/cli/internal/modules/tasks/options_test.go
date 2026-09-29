package tasks

import "testing"

func TestConcurrencyIsIndependentOfTaskName(t *testing.T) {
	for _, name := range []string{"dev", "serve", "build"} {
		p := &Plan{Tasks: []Task{{Name: "web", Run: "pnpm run serve"}, {Name: "api", Run: "task serve"}}}
		opts := Options{Name: name, Jobs: 1}
		if err := executionOptions(p, &opts); err != nil || opts.Jobs != 2 {
			t.Fatalf("%+v %v", opts, err)
		}
		opts = Options{Name: name, Jobs: 1, JobsExplicit: true}
		if err := executionOptions(p, &opts); err != nil || opts.Jobs != 1 {
			t.Fatal(opts, err)
		}
		p.Tasks[0].Interactive = true
		if err := executionOptions(p, &opts); err == nil {
			t.Fatal("concurrent interactive task accepted")
		}
	}
}

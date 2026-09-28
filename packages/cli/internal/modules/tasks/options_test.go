package tasks

import "testing"

func TestDevelopmentConcurrencyAndTerminalAccess(t *testing.T) {
	build := Task{Name: "lib:build", Operation: "build", Managed: true, Run: "one __task"}
	web := Task{Name: "web:dev", Operation: "dev", Managed: true, Run: "one __task"}
	api := Task{Name: "api:dev", Operation: "dev", Managed: true, Run: "one __task"}
	for _, tc := range []struct {
		name    string
		tasks   []Task
		opts    Options
		wantErr bool
		jobs    int
	}{
		{"automatic", []Task{build, web, api}, Options{Name: "dev", Jobs: 1}, false, 3},
		{"two slots", []Task{build, web, api}, Options{Name: "dev", Jobs: 2, JobsExplicit: true}, false, 2},
		{"starvation", []Task{build, web, api}, Options{Name: "dev", Jobs: 1, JobsExplicit: true}, true, 1},
		{"single raw with prerequisite", []Task{build, web}, Options{Name: "dev", Jobs: 1, UI: "raw"}, false, 2},
		{"multiple raw", []Task{web, api}, Options{Name: "dev", Jobs: 1, UI: "raw"}, true, 2},
		{"finite unchanged", []Task{build}, Options{Name: "build", Jobs: 1}, false, 1},
		{"custom services", []Task{{Operation: "web", Run: "server"}, {Operation: "api", Run: "server", Raw: true}}, Options{Name: "dev", Jobs: 1}, true, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := executionOptions(&Plan{Tasks: tc.tasks}, &tc.opts)
			if (err != nil) != tc.wantErr || tc.opts.Jobs != tc.jobs {
				t.Fatalf("opts=%+v err=%v", tc.opts, err)
			}
		})
	}
	web.Interactive = true
	if err := executionOptions(&Plan{Tasks: []Task{web, api}}, &Options{Name: "dev", Jobs: 1}); err == nil {
		t.Fatal("concurrent interactive service accepted")
	}
}

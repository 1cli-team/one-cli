package preset

import "sort"

// kindOrder gives each segment kind a stable sort key for canonical
// output. Project kinds come first in 'b' -> 'f' -> 'l' order (backend
// first matches both execution order — frontends consume backend ports
// — and ASCII order); env / x-* (workspace-level / extensions) trail.
var kindOrder = map[Kind]int{
	KindBackend:  0,
	KindFrontend: 1,
	KindLibrary:  2,
}

// canonicalize returns a new Spec with Items sorted into canonical
// order:
//  1. by kind (frontend before backend before library)
//  2. then by template code (ASCII)
//  3. then by deploy code (ASCII)
//
// The input is not mutated. UnknownSegments are sorted ASCII-wise.
func canonicalize(s Spec) Spec {
	out := Spec{
		Items:           make([]Item, len(s.Items)),
		EnvCode:         s.EnvCode,
		UnknownSegments: append([]string(nil), s.UnknownSegments...),
	}
	copy(out.Items, s.Items)
	sort.SliceStable(out.Items, func(i, j int) bool {
		a, b := out.Items[i], out.Items[j]
		if ko := kindOrder[a.Kind] - kindOrder[b.Kind]; ko != 0 {
			return ko < 0
		}
		if a.TemplateCode != b.TemplateCode {
			return a.TemplateCode < b.TemplateCode
		}
		if a.DeployCode != b.DeployCode {
			return a.DeployCode < b.DeployCode
		}
		return a.ContainerCode < b.ContainerCode
	})
	sort.Strings(out.UnknownSegments)
	return out
}

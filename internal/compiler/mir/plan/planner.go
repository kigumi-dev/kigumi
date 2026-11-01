package plan

import "kigumi/internal/compiler/mir"

// planner carries one program through the planning passes; their helpers are its methods.
type planner struct{ *mir.Program }

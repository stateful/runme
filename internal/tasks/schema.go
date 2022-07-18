package tasks

type TaskConfiguration struct {
	BaseTaskConfiguration

	Version string                 `json:"version" validate:"eq=2.0.0"`
	Windows *BaseTaskConfiguration `json:"windows,omitempty"`
	OSX     *BaseTaskConfiguration `json:"osx,omitempty"`
	Linux   *BaseTaskConfiguration `json:"linux,omitempty"`
}

type BaseTaskConfiguration struct {
	Type         *string              `json:"type,omitempty" validate:"omitempty,eq=shell|eq=process"`
	Command      *string              `json:"command,omitempty"`
	IsBackground *bool                `json:"isBackground,omitempty"`
	Options      *CommandOptions      `json:"options,omitempty"`
	Args         []string             `json:"args,omitempty"`
	Presentation *PresentationOptions `json:"presentation,omitempty"`
	// ProblemMatcher struct{}           `json:"problemMatcher,omitempty"`
	Tasks []TaskDescription `json:"tasks,omitempty"`
}

type CommandOptions struct {
	Cwd   string            `json:"cwd,omitempty"`
	Env   map[string]string `json:"env,omitempty"`
	Shell *struct {
		Executable string   `json:"executable" validate:"required"`
		Args       []string `json:"args,omitempty"`
	} `json:"shell,omitempty"`
}

type TaskDescription struct {
	Label        string               `json:"label" validate:"required"`
	Type         string               `json:"type,omitempty" validate:"eq=shell|eq=process"`
	Command      string               `json:"command" validate:"required"`
	IsBackground bool                 `json:"isBackground,omitempty"`
	Options      *CommandOptions      `json:"options,omitempty"`
	Args         []string             `json:"args,omitempty"`
	Group        string               `json:"group"`
	Presentation *PresentationOptions `json:"presentation,omitempty"`
	// ProblemMatcher struct{} `json:"problemMatcher,omitempty"`
	// RunOptions struct{} `json:"runOptions"`
}

// type Group interface {
// 	Object() GroupObj
// 	String() string
// }

// type GroupObj struct {
// 	Kind      string `json:"kind"`
// 	IsDefault bool   `json:"isDefault"`
// }

type PresentationOptions struct {
	Reveal           *string `json:"reveal" validate:"eq=never|eq=silent|eq=always"`
	Echo             *bool   `json:"echo"`
	Focus            *bool   `json:"focus"`
	Panel            *string `json:"panel" validate:"eq=shared|eq=dedicated|eq=new"`
	ShowReuseMessage *bool   `json:"showReuseMessage"`
	Clear            *bool   `json:"clear"`
	Group            *string `json:"group"`
}

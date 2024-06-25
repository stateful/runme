package owl

const ComplexSpecType string = "Complex"

type SpecDef struct {
	Name    string
	Breaker string
	Items   map[string]*varSpec
}

var SpecDefTypes = map[string]*SpecDef{
	"Redis": {
		Name:    "Redis",
		Breaker: "REDIS",
		Items: map[string]*varSpec{
			"HOST": {
				Name:     SpecNamePlain,
				Rules:    "required,min=4,max=32",
				Required: true,
			},
			"PORT": {
				Name:     SpecNamePlain,
				Rules:    "required,min=4,max=32",
				Required: true,
			},
			"PASSWORD": {
				Name:     SpecNamePassword,
				Rules:    "required,min=4,max=32",
				Required: false,
			},
		},
	},
	"Postgres": {
		Name:    "Postgres",
		Breaker: "POSTGRES",
		Items: map[string]*varSpec{
			"HOST": {
				Name:     SpecNamePlain,
				Rules:    "required,min=4,max=32",
				Required: true,
			},
		},
	},
}

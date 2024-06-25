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
				Required: true,
			},
			"PORT": {
				Name:     SpecNamePlain,
				Required: true,
			},
			"PASSWORD": {
				Name:     SpecNamePassword,
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
				Required: true,
			},
		},
	},
}

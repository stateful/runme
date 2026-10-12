package api

const (
	// TokenCreatorRole permits Google token minting within the broker's policy.
	TokenCreatorRole = "role/tokenCreator"
	// RunnerUserRole is the role for the runner user.
	RunnerUserRole = "role/runner.user"

	// AgentUserRole is the role for the agent user.
	AgentUserRole = "role/agent.user"

	// ParserUserRole is the role for the parser user.
	// Parser service is used for serialization.
	ParserUserRole = "role/parser.user"
)

type MemberKind string

const (
	UserKind   MemberKind = "user"
	DomainKind MemberKind = "domain"
)

// IAMPolicy is a policy that defines the access control for the service.
type IAMPolicy struct {
	// Bindings is a list of bindings that define the access control for the service.
	Bindings []IAMBinding `json:"bindings" yaml:"bindings"`
}

// IAMBinding is a binding of identities to roles.
// N.B. Currently we don't have any roles defined so this is just a list of members.
type IAMBinding struct {
	// Members is a list of members that are allowed to access the service
	Members []Member `json:"members" yaml:"members"`

	// Role is the role for the members
	Role string `json:"role" yaml:"role"`
}

type Member struct {
	// Name is the name of the member. e.g. jlewi@acme.com
	// N.B. In the future we could add a kind field to indicate what type of member it is
	// (e.g.domain, serviceaccount, group).
	Name string     `json:"name" yaml:"name"`
	Kind MemberKind `json:"kind" yaml:"kind"`
}

package iam

import (
	"fmt"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"github.com/pkg/errors"

	"github.com/runmedev/runme/v3/pkg/agent/api"
)

type Checker interface {
	Check(principal string, role string) bool
	GetPrincipal(idToken *jwt.Token) (string, error)
}

// PolicyChecker enforces IAMPolicies. It checks if a user has the required permissions to perform an action.
type PolicyChecker struct {
	policy api.IAMPolicy

	// Create a map from roles to matchers
	roles map[string][]memberMatcher
}

// NewChecker creates a new IAM policy checker.
func NewChecker(policy api.IAMPolicy) (*PolicyChecker, error) {
	// Validate the policy
	if isValid, msg := IsValidPolicy(policy); !isValid {
		return nil, errors.New(msg)
	}

	c := &PolicyChecker{
		policy: policy,
		roles:  make(map[string][]memberMatcher),
	}

	// Cache the roles
	for _, binding := range policy.Bindings {
		if _, ok := c.roles[binding.Role]; !ok {
			c.roles[binding.Role] = make([]memberMatcher, 0, 2)
		}

		userMatcher := &userChecker{
			members: make(map[string]bool),
		}

		for _, member := range binding.Members {
			switch member.Kind {
			case api.UserKind:
				userMatcher.members[member.Name] = true
			case api.DomainKind:
				domainMatcher := &domainMatcher{
					domain: member.Name,
				}
				c.roles[binding.Role] = append(c.roles[binding.Role], domainMatcher)
			default:
				return nil, errors.Errorf("member %s: kind must be one of: user, domain", member.Name)
			}
		}

		if len(userMatcher.members) > 0 {
			c.roles[binding.Role] = append(c.roles[binding.Role], userMatcher)
		}
	}

	return c, nil
}

// Check returns true if and only if the principal has the given role in the IAM policy.
func (c *PolicyChecker) Check(principal string, role string) bool {
	matchers, ok := c.roles[role]
	if !ok {
		return false
	}

	for _, m := range matchers {
		if m.Check(principal) {
			return true
		}
	}

	return false
}

// GetPrincipal returns the principal from the idToken.
func (c *PolicyChecker) GetPrincipal(idToken *jwt.Token) (string, error) {
	if idToken != nil && idToken.Valid {
		if claims, ok := idToken.Claims.(*staticClaims); ok {
			return claims.principal, nil
		}
	}
	email, err := extractEmailFromIDToken(idToken)
	if err != nil {
		return "", err
	}

	return email, nil
}

// IsValidPolicy checks if the IAM policy is valid. If its not it returns a string with a human readable
// message about the violations
func IsValidPolicy(policy api.IAMPolicy) (bool, string) {
	allowedRoles := map[string]bool{api.RunnerUserRole: true, api.AgentUserRole: true, api.ParserUserRole: true, api.TokenCreatorRole: true}
	roleNames := make([]string, 0, len(policy.Bindings))
	for name := range allowedRoles {
		roleNames = append(roleNames, name)
	}

	violations := func() []string {
		violations := make([]string, 0, 10)
		// Check if the policy is valid
		if len(policy.Bindings) == 0 {
			violations = append(violations, "policy must have at least one binding")
			return violations
		}

		for _, binding := range policy.Bindings {
			if len(binding.Members) == 0 {
				violations = append(violations, "binding must have at least one member")
			}

			if binding.Role == "" {
				violations = append(violations, "binding must have a role")
			}

			if _, ok := allowedRoles[binding.Role]; !ok {
				violations = append(violations, "binding role must be one of: %s", strings.Join(roleNames, ","))
			}

			for _, member := range binding.Members {
				if member.Kind != api.UserKind && member.Kind != api.DomainKind {
					violations = append(violations, fmt.Sprintf("member %s: kind must be one of: user, domain", member.Name))
				}
			}
		}

		return violations
	}()

	message := ""
	if len(violations) > 0 {
		message = "IAM policy is invalid. Violations: " + strings.Join(violations, ", ")
		return false, message
	}

	return true, message
}

// AllowAllChecker is a no auth checker that allows all requests.
type AllowAllChecker struct{}

func (c *AllowAllChecker) Check(principal string, role string) bool {
	return true
}

// GetPrincipal returns empty principal.
func (c *AllowAllChecker) GetPrincipal(idToken *jwt.Token) (string, error) {
	return "", nil
}

// memberMatcher checks whether a member is allowed under some set of rules
type memberMatcher interface {
	Check(principal string) bool
}

// userMatcher checks whether the principal is a member of a list of users
type userChecker struct {
	members map[string]bool
}

func (m *userChecker) Check(principal string) bool {
	_, ok := m.members[principal]
	return ok
}

// extractEmailFromIDToken extracts the email from the idToken, returning an error if any check fails.
func extractEmailFromIDToken(idToken *jwt.Token) (string, error) {
	if idToken == nil {
		return "", errors.New("No valid session")
	}
	claims, ok := idToken.Claims.(jwt.MapClaims)
	if !ok {
		return "", errors.New("Invalid token claims")
	}
	email, ok := claims["email"].(string)
	if !ok {
		return "", errors.New("Missing email claim")
	}
	return email, nil
}

package gitrepo

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"github.com/pkg/errors"
)

type Branch struct {
	Name        string
	Description string
}

func GetUsersBranchNames(cwd string, email string) ([]Branch, error) {
	// Get a list of branch names for a specific user within this repository
	// Due to limitations of go-git, we use shell for this.
	//
	// NB: user is _not_ sanitized, don't pass untrusted.
	//     Also due to weird exec.Command quoting, don't include whitespace
	//
	cmdSlice := []string{"git", "log", "--format=%s--||--%b", "--merges", "--author=" + strings.Trim(email, "\n")}
	return getBranchNamesFromCommand(cwd, cmdSlice, false)
}

func GetBranchNames(cwd string) ([]Branch, error) {
	cmdSlice := []string{"git", "log", "--format=%s--||--%b", "--merges"}
	return getBranchNamesFromCommand(cwd, cmdSlice, true)
}

func getBranchNamesFromCommand(cwd string, cmdSlice []string, greedy bool) ([]Branch, error) {
	if len(cmdSlice) < 2 {
		return nil, errors.New("command is not long enough")
	}
	cmd := exec.Command(cmdSlice[0], cmdSlice[1:]...)
	cmd.Dir = cwd
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, err
	}

	stdout := string(out)
	branches := getBranchNamesFromStdout(stdout, greedy)
	return branches, nil
}

func getBranchNamesFromStdout(stdout string, greedy bool) []Branch {
	var branches []Branch
	modifier := "(?mU)"
	if greedy {
		modifier = "(?m)"
	}
	re := regexp.MustCompile(fmt.Sprintf(`%s(\S+)[:\/](\S+)\s?$`, modifier))
	for _, line := range strings.Split(stdout, "\n") {
		split := strings.Split(line, "--||--")
		orgbranch := re.FindStringSubmatch(split[0])
		if len(orgbranch) == 3 && len(split) > 1 && len(split[1]) > 1 {
			branches = append(branches, Branch{Name: orgbranch[2], Description: split[1]})
		}
	}
	return branches
}

func GetUsersBranches(repoUser string) ([]Branch, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	var email string

	if len(repoUser) > 0 {
		email = repoUser
	} else {
		email, err = GetCurrentGitEmail(cwd)
		if err != nil {
			return nil, errors.New("could not find current git user")
		}
	}

	branches, err := GetUsersBranchNames(cwd, email)
	if err != nil {
		return nil, errors.New("error while querying user's branches")
	}

	return branches, nil
}

func GetRepoBranches() ([]Branch, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	branches, err := GetBranchNames(cwd)
	if err != nil {
		return nil, errors.New("error while querying repository branches")
	}

	return branches, nil
}

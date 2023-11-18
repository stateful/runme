package gitrepo

import "os/exec"

func GetCurrentGitEmail(cwd string) (string, error) {
	cmdSlice := []string{"git", "config", "user.email"}
	cmd := exec.Command(cmdSlice[0], cmdSlice[1:]...)
	cmd.Dir = cwd

	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", err
	}

	return string(out), nil
}

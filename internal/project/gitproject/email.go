package gitproject

import "os/exec"

// TODO: what if an email is set globally?
func GetCurrentGitEmail(cwd string) (string, error) {
	cmd := exec.Command("git", "config", "user.email")
	cmd.Dir = cwd

	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", err
	}

	return string(out), nil
}

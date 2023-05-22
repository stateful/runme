package executable

var supportedExecutables = []string{
	"bash",
	"bat", // fallback to sh
	"sh",
	"sh-raw",
	"shell",
	"zsh",
	"go",
}

func IsSupported(lang string) bool {
	for _, item := range supportedExecutables {
		if item == lang {
			return true
		}
	}
	return false
}

func IsShell(lang string) bool {
	return lang == "sh" || lang == "shell" || lang == "sh-raw"
}

package testutils

import "github.com/stateful/runme/internal/project"

var GitProjectLoadAllExpectedEvents = []project.LoadEventType{
	project.LoadEventStartedWalk,
	project.LoadEventFoundDir,  // "."
	project.LoadEventFoundFile, // "git-ignored.md"
	project.LoadEventFoundFile, // "ignored.md"
	project.LoadEventFoundFile, // "readme.md"
	project.LoadEventFinishedWalk,
	project.LoadEventStartedParsingDocument,  // "git-ignored.md"
	project.LoadEventFinishedParsingDocument, // "git-ignored.md"
	project.LoadEventFoundTask,
	project.LoadEventStartedParsingDocument,  // "ignored.md"
	project.LoadEventFinishedParsingDocument, // "ignored.md"
	project.LoadEventFoundTask,
	project.LoadEventStartedParsingDocument,  // "readme.md"
	project.LoadEventFinishedParsingDocument, // "readme.md"
	project.LoadEventFoundTask,
}

var GitProjectLoadOnlyNotIgnoredFilesEvents = []project.LoadEventType{
	project.LoadEventStartedWalk,
	project.LoadEventFoundDir,  // "."
	project.LoadEventFoundFile, // "readme.md"
	project.LoadEventFinishedWalk,
	project.LoadEventStartedParsingDocument,  // "readme.md"
	project.LoadEventFinishedParsingDocument, // "readme.md"
	project.LoadEventFoundTask,
}

var FileProjectEvents = []project.LoadEventType{
	project.LoadEventStartedWalk,
	project.LoadEventFoundFile, // "file-project.md"
	project.LoadEventFinishedWalk,
	project.LoadEventStartedParsingDocument,  // "file-project.md"
	project.LoadEventFinishedParsingDocument, // "file-project.md"
	project.LoadEventFoundTask,
}

func IgnoreFilePatternsWithDefaults(patterns ...string) []string {
	return append([]string{"*.bkp"}, patterns...)
}

package testutils

import "github.com/stateful/runme/v3/pkg/project"

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
	project.LoadEventFoundTask,               // unnamed; echo-hello
	project.LoadEventFoundTask,               // named; my-task
}

var GitProjectLoadOnlyNotIgnoredFilesEvents = []project.LoadEventType{
	project.LoadEventStartedWalk,
	project.LoadEventFoundDir,  // "."
	project.LoadEventFoundDir,  // "nested"
	project.LoadEventFoundFile, // "readme.md"
	project.LoadEventFinishedWalk,
	project.LoadEventStartedParsingDocument,  // "readme.md"
	project.LoadEventFinishedParsingDocument, // "readme.md"
	project.LoadEventFoundTask,               // unnamed; echo-hello
	project.LoadEventFoundTask,               // named; my-task
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

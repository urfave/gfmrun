package gfmxr

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/Sirupsen/logrus"
)

var (
	rawTagsRe = regexp.MustCompile("<!-- *({.+}) *-->")

	codeGateCharsRe = regexp.MustCompile("[`~]+")
)

type mdState int

func (s mdState) String() string {
	switch s {
	case mdStateText:
		return "text"
	case mdStateCodeBlock:
		return "code-block"
	case mdStateRunnable:
		return "runnable"
	case mdStateComment:
		return "comment"
	default:
		return "unknown"
	}
}

const (
	mdStateText mdState = iota
	mdStateCodeBlock
	mdStateRunnable
	mdStateComment
)

var (
	mdStateTransTextCodeBlock     = calcStateTransition(mdStateText, mdStateCodeBlock)
	mdStateTransTextRunnable      = calcStateTransition(mdStateText, mdStateRunnable)
	mdStateTransTextComment       = calcStateTransition(mdStateText, mdStateComment)
	mdStateTransCodeBlockText     = calcStateTransition(mdStateCodeBlock, mdStateText)
	mdStateTransCodeBlockRunnable = calcStateTransition(mdStateCodeBlock, mdStateRunnable)
	mdStateTransCodeBlockComment  = calcStateTransition(mdStateCodeBlock, mdStateComment)
	mdStateTransRunnableText      = calcStateTransition(mdStateRunnable, mdStateText)
	mdStateTransRunnableCodeBlock = calcStateTransition(mdStateRunnable, mdStateCodeBlock)
	mdStateTransRunnableComment   = calcStateTransition(mdStateRunnable, mdStateComment)
	mdStateTransCommentText       = calcStateTransition(mdStateComment, mdStateText)
	mdStateTransCommentCodeBlock  = calcStateTransition(mdStateComment, mdStateCodeBlock)
	mdStateTransCommentRunnable   = calcStateTransition(mdStateComment, mdStateRunnable)
)

func calcStateTransition(a, b mdState) int {
	return int(a | (b << 2))
}

type runnableFinder struct {
	sourceName string
	source     string
	log        *logrus.Logger

	state mdState

	cur *Runnable

	line           string
	trimmedLine    string
	lineno         int
	textSize       int
	lastLine       string
	lastComment    string
	codeBlockStart string
}

// custom markdown scanner thingy egad
// (because blackfriday doesn't have all the things and/or I'm horrible)
func newRunnableFinder(sourceName, source string, log *logrus.Logger) *runnableFinder {
	rf := &runnableFinder{sourceName: sourceName, source: source, log: log}
	rf.reset()
	return rf
}

func (rf *runnableFinder) reset() {
	rf.cur = NewRunnable(rf.sourceName, rf.log)
	rf.state = mdStateText
	rf.line = ""
	rf.trimmedLine = ""
	rf.codeBlockStart = ""
	rf.lineno = 0
	rf.lastLine = ""
	rf.lastComment = ""
}

func (rf *runnableFinder) Find() []*Runnable {
	rf.reset()
	runnables := []*Runnable{}

	for j, line := range strings.Split(rf.source, "\n") {
		rf.line = line
		rf.lineno = j
		rf.lastLine = line
		rf.trimmedLine = strings.TrimSpace(rf.line)

		runnable := rf.handleLine()
		if runnable != nil {
			runnables = append(runnables, runnable)
			rf.log.WithField("runnable_count", len(runnables)).Debug("leaving runnable code block")
		}

		rf.log.WithFields(logrus.Fields{
			"source_name": rf.sourceName,
			"lineno":      rf.lineno,
			"line":        fmt.Sprintf("%q", rf.trimmedLine),
			"state":       rf.state,
		}).Debug("scanning")
	}

	if rf.state == mdStateRunnable {
		// whatever, let's give it a shot
		rf.cur.Lines = append(rf.cur.Lines, rf.lastLine)
		runnables = append(runnables, rf.cur)
	}

	return runnables
}

func (rf *runnableFinder) handleLine() *Runnable {
	if strings.HasPrefix(rf.trimmedLine, "```") || strings.HasPrefix(rf.trimmedLine, "~~~") {
		if rf.state == mdStateCodeBlock {
			if rf.trimmedLine == rf.codeBlockStart {
				return rf.setState(mdStateText)
			} else {
				rf.log.Debug("assuming nested code block")
				return nil
			}
		}

		if rf.state == mdStateRunnable {
			if rf.trimmedLine == rf.cur.BlockStart {
				// assuming this is the matching closing gate of the runnable code block
				return rf.setState(mdStateText)
			} else {
				rf.log.WithFields(logrus.Fields{
					"block_start": rf.cur.BlockStart,
					"lang":        rf.cur.Lang,
					"line":        rf.trimmedLine,
				}).Debug("mismatched closing gate")
			}
		}

		if len(codeGateCharsRe.ReplaceAllString(rf.trimmedLine, "")) > 0 {
			return rf.setState(mdStateRunnable)
		}

		return rf.setState(mdStateCodeBlock)
	} else if strings.HasPrefix(rf.trimmedLine, "<!--") && strings.HasSuffix(rf.trimmedLine, "-->") {
		if rf.state == mdStateText {
			rf.setState(mdStateComment)
			rf.line = ""
			rf.trimmedLine = ""
			rf.setState(mdStateText)
		} else {
			rf.log.WithFields(logrus.Fields{
				"state": rf.state,
				"line":  rf.trimmedLine,
			}).Debug("not setting lastComment")
		}
	} else if strings.HasPrefix(rf.trimmedLine, "<!--") {
		return rf.setState(mdStateComment)
	} else if strings.HasPrefix(rf.trimmedLine, "-->") && rf.state == mdStateComment {
		rf.trimmedLine = strings.TrimSpace(strings.Replace(rf.trimmedLine, "-->", "", 1))
		return rf.setState(mdStateText)
	}

	rf.handleLineInState()
	return nil
}

func (rf *runnableFinder) setState(newState mdState) *Runnable {
	oldState := rf.state
	rf.state = newState

	transition := calcStateTransition(oldState, newState)
	rf.log.WithFields(logrus.Fields{
		"transition": transition,
		"old_state":  oldState,
		"new_state":  newState,
	}).Debug("setting state")

	return rf.handleTransition(transition)
}

func (rf *runnableFinder) handleTransition(transition int) *Runnable {
	switch transition {
	case mdStateTransCodeBlockComment:
		rf.log.WithFields(logrus.Fields{
			"state":         mdStateCodeBlock,
			"invalid_state": mdStateComment,
		}).Debug("ignoring transition")
		rf.state = mdStateCodeBlock
	case mdStateTransRunnableCodeBlock:
		rf.log.WithFields(logrus.Fields{
			"state":         mdStateRunnable,
			"invalid_state": mdStateCodeBlock,
		}).Debug("ignoring transition")
		rf.state = mdStateRunnable
	case mdStateTransCodeBlockRunnable:
		rf.log.WithFields(logrus.Fields{
			"state":         mdStateCodeBlock,
			"invalid_state": mdStateRunnable,
		}).Debug("ignoring transition")
		rf.state = mdStateCodeBlock
	case mdStateTransTextCodeBlock:
		rf.codeBlockStart = rf.trimmedLine
		rf.textSize = 0
	case mdStateTransTextComment:
		rf.textSize = 0
		rf.lastComment = rf.line
	case mdStateTransCommentText:
		rf.textSize = len(rf.trimmedLine)
	case mdStateTransCodeBlockText:
		rf.codeBlockStart = ""
		rf.log.Debug("leaving non-runnable code block")
	case mdStateTransRunnableText:
		runnable := rf.cur
		rf.cur = NewRunnable(rf.sourceName, rf.log)
		rf.lastComment = ""
		return runnable
	case mdStateTransTextCodeBlock:
		rf.log.WithField("lineno", rf.lineno).Debug("starting new non-runnable code block")
		rf.lastComment = ""
	case mdStateTransTextRunnable, mdStateTransCommentRunnable:
		rf.log.WithFields(logrus.Fields{
			"lineno":       rf.lineno,
			"text_size":    rf.textSize,
			"last_comment": rf.lastComment,
		}).Debug("starting new runnable")

		// textSize of 0 means that the last comment is adjacent to the runnable
		if rf.textSize == 0 {
			trimmedComment := rawTagsRe.FindStringSubmatch(strings.TrimSpace(rf.lastComment))
			if len(trimmedComment) > 1 {
				rf.log.WithField("raw_tags", trimmedComment[1]).Debug("setting raw tags")
				rf.cur.RawTags = trimmedComment[1]
			}
		}

		rf.lastComment = ""
		rf.cur.Begin(rf.lineno, rf.trimmedLine)
	default:
		rf.log.WithField("transition", transition).Debug("unhandled transition")
	}
	return nil
}

func (rf *runnableFinder) handleLineInState() {
	switch rf.state {
	case mdStateComment:
		rf.lastComment += rf.line
	case mdStateRunnable:
		rf.cur.Lines = append(rf.cur.Lines, rf.line)
	case mdStateText:
		rf.textSize += len(rf.trimmedLine)
	}
}

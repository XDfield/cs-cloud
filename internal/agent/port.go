package agent

import "regexp"

var PortPatterns = []*regexp.Regexp{
	regexp.MustCompile(`listening on https?://[\d.:]+:(\d+)`),
	regexp.MustCompile(`internal server on port (\d+)`),
	regexp.MustCompile(`listening on .*?:(\d+)`),
}

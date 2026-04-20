package agent

import "regexp"

var PortPatterns = []*regexp.Regexp{
	regexp.MustCompile(`listening on http://[\d.:]+:(\d+)`),
	regexp.MustCompile(`internal server on port (\d+)`),
	regexp.MustCompile(`server listening on .+:(\d+)`),
}

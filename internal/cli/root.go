package cli

import (
	"fmt"
	"os"
	"strings"

	"cs-cloud/internal/app"
	"cs-cloud/internal/platform"
)

func Execute() error {
	a, err := app.New()
	if err != nil {
		return err
	}

	parseGlobalFlags()

	return dispatch(a)
}

func parseGlobalFlags() {
	args := os.Args[1:]
	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "--auth-path" && i+1 < len(args):
			platform.SetAuthPath(args[i+1])
			i++
		case len(args[i]) > 11 && args[i][:11] == "--auth-path=":
			platform.SetAuthPath(args[i][11:])
		}
	}
}

func commandArgs() []string {
	var rest []string
	args := os.Args[1:]
	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "--auth-path" && i+1 < len(args):
			i++
		case len(args[i]) > 11 && args[i][:11] == "--auth-path=":
		default:
			rest = append(rest, args[i])
		}
	}
	return rest
}

func parseMode() string {
	args := os.Args[1:]
	for i := 0; i < len(args); i++ {
		switch {
		case (args[i] == "--mode" || args[i] == "-m") && i+1 < len(args):
			return args[i+1]
		case strings.HasPrefix(args[i], "--mode="):
			return args[i][7:]
		case strings.HasPrefix(args[i], "-m="):
			return args[i][3:]
		}
	}
	return "cloud"
}

func dispatch(a *app.App) error {
	cmds := commandArgs()
	if len(cmds) == 0 {
		printUsage()
		return nil
	}

	switch cmds[0] {
	case "start":
		return start(a)
	case "stop":
		return stop(a)
	case "restart":
		return restart(a)
	case "status":
		return status(a)
	case "logs":
		return logs(a)
	case "doctor":
		return doctor(a)
	case "register":
		return register(a)
	case "login":
		return login(a)
	case "logout":
		return logout(a)
	case "serve":
		return serve(a)
	case "_daemon":
		return runDaemon(a)
	case "help", "-h", "--help":
		printUsage()
		return nil
	default:
		printUsage()
		return fmt.Errorf("unknown command: %s", cmds[0])
	}
}

func printUsage() {
	fmt.Println("Usage: cs-cloud [flags] <command>")
	fmt.Println()
	fmt.Println("Flags:")
	fmt.Println("  --auth-path <path>  Path to auth.json (default: ~/.costrict/share/auth.json)")
	fmt.Println("  --mode, -m <mode>   Daemon mode: cloud (default) or local")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  start     Start daemon (cloud mode with WS tunnel, or --mode local for HTTP only)")
	fmt.Println("  stop      Stop daemon")
	fmt.Println("  restart   Restart daemon")
	fmt.Println("  status    Show daemon status")
	fmt.Println("  logs      Show daemon logs")
	fmt.Println("  doctor    Show diagnostic info")
	fmt.Println("  register  Register device")
	fmt.Println("  login     Login via browser OAuth")
	fmt.Println("  logout    Delete credentials and device info")
	fmt.Println("  serve     Run server in foreground (no daemon)")
}

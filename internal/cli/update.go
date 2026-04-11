package cli

import (
	"context"
	"fmt"

	"cs-cloud/internal/app"
	"cs-cloud/internal/updater"
	"cs-cloud/internal/version"
)

func updateCmd(a *app.App) error {
	args := commandArgs()
	if len(args) < 2 {
		printUpdateUsage()
		return fmt.Errorf("missing update subcommand")
	}

	mgr := updater.NewManager(a.CloudBaseURL(), a.RootDir())

	switch args[1] {
	case "check":
		return updateCheck(mgr)
	case "apply":
		targetVer := ""
		for i := 2; i < len(args); i++ {
			if args[i] == "--version" && i+1 < len(args) {
				targetVer = args[i+1]
				break
			}
		}
		return updateApply(mgr, targetVer)
	case "rollback":
		return updateRollback(mgr)
	case "history":
		return updateHistory(mgr)
	default:
		printUpdateUsage()
		return fmt.Errorf("unknown update subcommand: %s", args[1])
	}
}

func updateCheck(mgr *updater.Manager) error {
	printTitle("cs-cloud update check")
	result, err := mgr.CheckNow(context.Background())
	if err != nil {
		printError("Check failed: %v", err)
		return err
	}

	if !result.Available {
		printSuccess("Already up to date")
		printKV("current", version.Get())
		return nil
	}

	printInfo("Update available")
	fmt.Print(renderKV([][2]string{
		{"current", version.Get()},
		{"latest", result.Version},
		{"changelog", result.Changelog},
		{"force", fmt.Sprintf("%v", result.Force)},
		{"platform", updater.PlatformString()},
	}))
	return nil
}

func updateApply(mgr *updater.Manager, targetVersion string) error {
	printTitle("cs-cloud update apply")
	if targetVersion != "" {
		printInfo("Target version: %s", targetVersion)
	}
	printInfo("Checking for updates...")

	if err := mgr.Apply(context.Background(), targetVersion); err != nil {
		printError("Upgrade failed: %v", err)
		return err
	}

	printSuccess("Upgrade completed")
	printInfo("Restart the daemon to use the new version")
	return nil
}

func updateRollback(mgr *updater.Manager) error {
	printTitle("cs-cloud update rollback")
	if err := mgr.Rollback(); err != nil {
		printError("Rollback failed: %v", err)
		return err
	}
	printSuccess("Rollback completed")
	printInfo("Restart the daemon to use the previous version")
	return nil
}

func updateHistory(mgr *updater.Manager) error {
	printTitle("cs-cloud update history")

	state, _ := mgr.History()
	if state != nil {
		printSection("Current")
		fmt.Print(renderKV([][2]string{
			{"previous", state.PreviousVersion},
			{"current", state.CurrentVersion},
			{"upgraded", state.UpgradedAt},
			{"status", state.Status},
			{"backup", state.BackupPath},
		}))
	}

	history, err := mgr.FullHistory()
	if err != nil {
		return err
	}
	if len(history) == 0 && state == nil {
		printInfo("No upgrade history")
		return nil
	}

	if len(history) > 0 {
		printSection("History")
		for i, h := range history {
			label := fmt.Sprintf("#%d", i+1)
			fmt.Print(renderKV([][2]string{
				{label + " version", fmt.Sprintf("%s → %s", h.PreviousVersion, h.CurrentVersion)},
				{label + " date", h.UpgradedAt},
				{label + " status", h.Status},
			}))
		}
	}
	return nil
}

func printUpdateUsage() {
	printSection("Update Commands")
	fmt.Print(renderKV([][2]string{
		{"update check", "Check for available updates"},
		{"update apply [--version v1.x.x]", "Apply available update"},
		{"update rollback", "Rollback to previous version"},
		{"update history", "Show upgrade history"},
	}))
}

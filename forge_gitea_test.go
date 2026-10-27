//go:build !noforge

package main

import "testing"

func TestGiteaLegacyTransferPublicCommands(t *testing.T) { runV1LegacyTransfer(t, "gitea", "1.27.3") }
func TestGiteaLegacyCapabilitiesAndLabelRefusal(t *testing.T) {
	runV1LegacyCapabilitiesAndLabelRefusal(t, "gitea", "1.27.3")
}

package starportcmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/tendermint/starport/starport/pkg/cosmosaccount"
)

func NewAccountExport() *cobra.Command {
	c := &cobra.Command{
		Use:   "export [name]",
		Short: "Export an account",
		Args:  cobra.ExactArgs(1),
		RunE:  accountExportHandler,
	}

	c.Flags().AddFlagSet(flagSetKeyringBackend())
	c.Flags().AddFlagSet(flagSetAccountImportExport())
	c.Flags().String(flagPath, "", "path to export private key. default: ./key_[name]")

	return c
}

func accountExportHandler(cmd *cobra.Command, args []string) error {
	var (
		name    = args[0]
		path, _ = cmd.Flags().GetString(flagPath)
	)

	passphrase, err := getPassphrase(cmd)
	if err != nil {
		return err
	}

	ca, err := cosmosaccount.New(getKeyringBackend(cmd))
	if err != nil {
		return err
	}

	armored, err := ca.Export(name, passphrase)
	if err != nil {
		return err
	}

	if path == "" {
		path = fmt.Sprintf("./key_%s", name)
	}
	path, err = filepath.Abs(path)
	if err != nil {
		return err
	}

	if err := os.WriteFile(path, []byte(armored), 0644); err != nil {
		return err
	}

	fmt.Printf("Account %q exported to file: %s\n", name, path)
	return nil
}

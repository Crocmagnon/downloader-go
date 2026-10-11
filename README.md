# downloader-go

Download your invoices, bank statements, etc.

Currently only supports the following:
* Freebox latest invoice
* Free mobile latest invoice
* Eau du Grand Lyon latest invoice
* Octopus Energy proof of address (non-functional)
* Shiva latest payslip
* LCL latest bank statement

## Usage

```console
Usage: downloader <command> [flags]

Flags:
  -h, --help                 Show context-sensitive help.
  -o, --output-dir=STRING    Output directory.
      --headless             Enable headless mode.
      --no-interaction       Enable interaction-less mode. In this mode,
                             if a user interaction is required, it will generate
                             an error instead.

Commands:
  freebox --username=STRING --password=STRING [flags]
    Download latest invoice from Freebox.

  free-mobile --username=STRING --password=STRING [flags]
    Download latest invoice from Free mobile.

  eau-du-grand-lyon --username=STRING --password=STRING [flags]
    Download latest invoice from Eau du Grand Lyon.

  octopus-energy-address --username=STRING --password=STRING [flags]
    Download latest proof of address from Octopus Energy.

  shiva --username=STRING --password=STRING [flags]
    Download latest payslip from Shiva.

  lcl-checking --username=STRING --password=STRING [flags]
    Download latest bank statement from LCL.

  recettes-cabas recap --username=STRING --password=STRING --config=STRING --telegram-token=STRING --telegram-chat-id=INT-64 [flags]
    Send the weekly recap to Telegram.

  recettes-cabas bot --username=STRING --password=STRING --config=STRING --telegram-token=STRING --telegram-chat-id=INT-64 [flags]
    Run the Telegram bot that applies selection changes.

  photos [flags]
    Download photos.

  version [flags]
    Print version information.

Run "downloader <command> --help" for more information on a command.
```

## Development

When updating go dependencies, remember to bump the vendorHash in flake.nix.

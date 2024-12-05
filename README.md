# downloader-go

Download your invoices, bank statements, etc.

Currently only supports the following:
* Freebox latest invoice
* Free mobile latest invoice

## Usage

```console
$ ./downloader -h
Usage: downloader --output-dir=STRING <command> [flags]

Flags:
  -h, --help                 Show context-sensitive help.
  -o, --output-dir=STRING    Output directory.
      --headless             Enable headless mode.
      --no-interaction       Enable interaction-less mode. In this mode, if a user interaction is required, it will generate
                             an error instead.

Commands:
  freebox --output-dir=STRING --username=STRING --password=STRING [flags]
    Download latest Freebox invoice.

  free-mobile --output-dir=STRING --username=STRING --password=STRING [flags]
    Download latest Free mobile invoice.

Run "downloader <command> --help" for more information on a command.
```
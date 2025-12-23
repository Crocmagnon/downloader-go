{
  description = "Playwright development environment";
  inputs.flake-utils.url = "github:numtide/flake-utils";
  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixos-25.11";
  inputs.playwright.url = "github:pietdevries94/playwright-web-flake/1.52.0";

  outputs =
    {
      self,
      flake-utils,
      nixpkgs,
      playwright,
    }:
    flake-utils.lib.eachDefaultSystem (
      system:
      let
        overlay = final: prev: {
          inherit (playwright.packages.${system}) playwright-test playwright-driver;
        };
        pkgs = import nixpkgs {
          inherit system;
          overlays = [ overlay ];
        };
      in
      {
        devShells.default = pkgs.mkShell {
          packages = with pkgs; [
            nodejs
            golangci-lint
          ];
          shellHook = ''
            export PLAYWRIGHT_SKIP_BROWSER_DOWNLOAD=1
            export PLAYWRIGHT_BROWSERS_PATH="${pkgs.playwright-driver.browsers}"
            export PLAYWRIGHT_DRIVER_PATH="${pkgs.playwright-driver}"
            export PLAYWRIGHT_NODEJS_PATH="${pkgs.nodejs}/bin/node"
          '';
        };
      }
    );
}

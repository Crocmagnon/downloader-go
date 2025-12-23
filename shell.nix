let
  nixpkgs = fetchTarball "https://github.com/NixOS/nixpkgs/tarball/nixos-25.11";
  playwright = fetchTarball "https://github.com/pietdevries94/playwright-web-flake/tarball/1.52.0";
  overlay = final: prev: {
    playwright-driver = pkgs.callPackage "${playwright}/playwright-driver" { };
  };
  pkgs = import nixpkgs {
    config = { };
    overlays = [ overlay ];
  };
in
pkgs.mkShellNoCC {
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
}

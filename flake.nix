{
  description = "A simple Go package";

  inputs.nixpkgs.url = "nixpkgs/nixos-26.05";
  # nixpkgs is pinned for playwright-driver so we have the proper version
  inputs.nixpkgs-pw.url = "github:NixOS/nixpkgs/f2676046a1cba86d6f86a64f5b6d427b2f4dec96";

  outputs =
    {
      self,
      nixpkgs,
      nixpkgs-pw,
    }:
    let
      version = "latest";
      supportedSystems = [
        "x86_64-linux"
        "x86_64-darwin"
        "aarch64-linux"
        "aarch64-darwin"
      ];
      forAllSystems = nixpkgs.lib.genAttrs supportedSystems;
      nixpkgsFor = forAllSystems (
        system:
        import nixpkgs {
          inherit system;
          overlays = [
            (final: prev: {
              inherit (nixpkgs-pw.legacyPackages.${system}) playwright-driver;
            })
          ];
        }
      );
    in
    {
      packages = forAllSystems (
        system:
        let
          pkgs = nixpkgsFor.${system};
        in
        {
          # go.mod requires Go 1.27 (encoding/json/v2).
          downloader-go = (pkgs.buildGoModule.override { go = pkgs.go_1_27; }) {
            pname = "downloader-go";
            inherit version;
            # In 'nix develop', we don't need a copy of the source tree
            # in the Nix store.
            src = ./.;

            # This hash locks the dependencies of this package. It is
            # necessary because of how Go requires network access to resolve
            # VCS.  See https://www.tweag.io/blog/2021-03-04-gomod2nix/ for
            # details. Normally one can build with a fake hash and rely on native Go
            # mechanisms to tell you what the hash should be or determine what
            # it should be "out-of-band" with other tooling (eg. gomod2nix).
            # To begin with it is recommended to set this, but one must
            # remember to bump this hash when your dependencies change.
            # vendorHash = pkgs.lib.fakeHash;

            vendorHash = "sha256-9fLTfvPiG43xE5t6iIvXBMUmCvd2tYMIzfia3Anw6SA=";

            nativeBuildInputs = [ pkgs.makeWrapper ];

            postInstall = ''
              mkdir -p $out/share/playwright/package
              ln -s ${pkgs.playwright-driver}/cli.js $out/share/playwright/package/cli.js

              wrapProgram $out/bin/downloader-go \
                --set PLAYWRIGHT_BROWSERS_PATH "${pkgs.playwright-driver.browsers}" \
                --set PLAYWRIGHT_DRIVER_PATH "$out/share/playwright" \
                --set PLAYWRIGHT_NODEJS_PATH "${pkgs.nodejs}/bin/node"
            '';
          };
        }
      );

      # Add dependencies that are only needed for development
      devShells = forAllSystems (
        system:
        let
          pkgs = nixpkgsFor.${system};
        in
        {
          default = pkgs.mkShellNoCC {
            buildInputs = with pkgs; [
              golangci-lint
              nodejs
            ];
            shellHook = ''
              export PLAYWRIGHT_BROWSERS_PATH="${pkgs.playwright-driver.browsers}"
              export PLAYWRIGHT_DRIVER_PATH="$PWD/.playwright"
              export PLAYWRIGHT_NODEJS_PATH="${pkgs.nodejs}/bin/node"
              mkdir -p "$PLAYWRIGHT_DRIVER_PATH/package"
              ln -sf "${pkgs.playwright-driver}/cli.js" "$PLAYWRIGHT_DRIVER_PATH/package/cli.js"
            '';
          };
        }
      );

      # The default package for 'nix build'. This makes sense if the
      # flake provides only one package or there is a clear "main"
      # package.
      defaultPackage = forAllSystems (system: self.packages.${system}.downloader-go);
    };
}

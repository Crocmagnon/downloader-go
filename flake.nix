{
  description = "A simple Go package";

  inputs.nixpkgs.url = "nixpkgs/nixos-25.11";
  inputs.nixpkgs-pw.url = "github:NixOS/nixpkgs/979daf34c8cacebcd917d540070b52a3c2b9b16e";

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
          downloader-go = pkgs.buildGoModule {
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

            vendorHash = "sha256-6n0QsQVxZXD33wMIxbgnduoQiW+elvIUYnCWhCRmQl4=";
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

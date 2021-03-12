{
  description = "go-capataz project's flake";

  # nixConfig.bash-prompt-suffix = "(flake)";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";
    flake-utils.url = "github:numtide/flake-utils";
    gomod2nix.url = "github:tweag/gomod2nix";
    gomod2nix.inputs.nixpkgs.follows = "nixpkgs";
  };

  outputs = { self, nixpkgs, gomod2nix, flake-utils }:
    flake-utils.lib.eachDefaultSystem (
      system:
      let
        pkgs = nixpkgs.legacyPackages.${system}.appendOverlays [
          self.overlay.${system}
          gomod2nix.overlay.${system}
        ];
        go = pkgs.go_1_16;
      in
        {

          defaultPackage =
            self.packages.${system}.go-capataz;

          packages =
            {

              humanlog =
                with pkgs; buildGoPackage rec {
                  name = "humanlog";
                  version = "0.5.0";
                  rev = "0.5.0";
                  goPackagePath = "github.com/aybabtme/humanlog";
                  src = fetchFromGitHub {
                    inherit rev;
                    owner = "aybabtme";
                    repo = "humanlog";
                    sha256 = "Ffq/dKu3yqGoy+oo3rCf54/1XYHRN9D6LYIPXMdG9rA=";
                  };
                  meta = with lib; {
                    description = "Logs for humans to read.";
                    homepage = "https://github.com/aybabtme/humanlog";
                    license = licenses.asl20;
                    # platforms = platforms.x86_64;
                  };
                };

              go-capataz =
                import ./nix/default.nix { inherit pkgs go; };

              ci-env =
                import ./nix/ci.nix { inherit pkgs go; };

            };

          overlay =
            final: prev: self.packages.${system};

          devShell =
            import ./nix/shell.nix { inherit pkgs go; };

        } # end outputs
    ); # end eachDefaultSystem
}

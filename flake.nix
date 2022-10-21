{
  description = "go-capataz project's flake";

  # nixConfig.bash-prompt-suffix = "(flake)";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";
    flake-utils.url = "github:numtide/flake-utils";
    gomod2nix.url = "github:tweag/gomod2nix";
    gomod2nix.inputs.nixpkgs.follows = "nixpkgs";
  };

  outputs = { self, nixpkgs, gomod2nix, flake-utils } @ inputs:
    flake-utils.lib.eachDefaultSystem (
      system:
      let
        pkgs = import nixpkgs {
          inherit system;
          overlays = [
            gomod2nix.overlays.default
          ];
        };
      in
        {

          packages =
            {
              default = self.packages.${system}.go-capataz;

              humanlog =
                pkgs.buildGoPackage rec {
                  name = "humanlog";
                  version = "0.6.0";
                  rev = "0.6.0";
                  goPackagePath = "github.com/aybabtme/humanlog";
                  src = pkgs.fetchFromGitHub {
                    inherit rev;
                    owner = "humanlogio";
                    repo = "humanlog";
                    sha256 = "sha256-oaGqrGLvMq0To8fBV1sgRnC21xsaxG0fgVSX22Ibuvw=";
                  };
                  meta = with pkgs.lib; {
                    description = "Logs for humans to read.";
                    homepage = "https://github.com/humanlogio/humanlog";
                    license = licenses.asl20;
                    platforms = platforms.x86_64 ++ platforms.aarch64;
                  };
                };

              go-capataz =
                pkgs.callPackage (import ./nix/default.nix system inputs) {};

            };

          devShells = {
            default = pkgs.callPackage (import ./nix/shell.nix system inputs) {};
            ci-env  = pkgs.callPackage (import ./nix/ci-env.nix system inputs) {};

          };

        } # end outputs
    ); # end eachDefaultSystem
}

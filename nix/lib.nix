{
  self,
  flake-utils,
  nixpkgs,
  pre-commit-hooks,
  ...
} @ inputs: {
  getPkgs = system:
    import nixpkgs {
      inherit system;
      overlays = builtins.attrValues self.overlays;
    };

  eachDefaultSystemWithPkgs = f:
    flake-utils.lib.eachDefaultSystemMap (system: f (self.lib.getPkgs system));

  importDerivationsDir = inputs: path: pkgs: let
    system = pkgs.system;

    subDirNames =
      builtins.attrNames
      (pkgs.lib.filterAttrs (name: ty: ty == "directory") (builtins.readDir path));

    # Get all the packages defined in the project
    allPkgs = builtins.foldl' (acc: pkgName:
      acc
      // {
        "${pkgName}" =
          pkgs.callPackage (import (path + "/${pkgName}") system inputs) {};
      }) {}
    subDirNames;
  in
    allPkgs;

  pre-commit = system: let
    pkgs = self.lib.getPkgs system;

    config =
      builtins.removeAttrs
      (pkgs.callPackage (import ./pre-commit.nix system inputs) {})
      ["override" "overrideDerivation"];
  in
    pre-commit-hooks.lib.${system}.run config;
}

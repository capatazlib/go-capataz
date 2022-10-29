{
  description = "go-capataz project's flake";

  # nixConfig.bash-prompt-suffix = "(flake)";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";
    flake-utils.url = "github:numtide/flake-utils";

    gomod2nix = {
      url = "github:tweag/gomod2nix";
      inputs.nixpkgs.follows = "nixpkgs";
      inputs.utils.follows = "flake-utils";
    };

    pre-commit-hooks = {
      url = "github:cachix/pre-commit-hooks.nix";
      inputs.nixpkgs.follows = "nixpkgs";
      inputs.flake-utils.follows = "flake-utils";
    };
  };

  outputs = {
    self,
    gomod2nix,
    ...
  } @ inputs: {
    lib = import ./nix/lib.nix inputs;

    overlays = {
      gomod2nix = gomod2nix.overlays.default;
    };

    packages =
      self.lib.eachDefaultSystemWithPkgs
      (self.lib.importDerivationsDir inputs ./nix/packages);

    devShells =
      self.lib.eachDefaultSystemWithPkgs
      (self.lib.importDerivationsDir inputs ./nix/devShells);
  };
}

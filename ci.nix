let
  sources = import ./nix/sources.nix {};

  # build an overlay with specific packages from sources.nix
  overlay = _: pkgs: {
    niv = import sources.niv {};
  };

  # get nixpkgs with pinned packages overlay
  pinnedPkgs = import sources.nixpkgs {
    overlays = [overlay];
  };
in

# This allows overriding pkgs by passing `--arg pkgs ...`
{ pkgs ? pinnedPkgs }:

let
  go-capataz = import ./default.nix { pkgs = pkgs; };

in
  pkgs.mkShell {
    GOROOT = go_1_14.GOROOT;
    buildInputs = with pkgs; [
      # current go version
      go_1_14
      go-capataz

      # linting
      gotools godef golint golangci-lint
    ];

    shellHook = ''
      unset GOROOT
      unset GOPATH
    '';
  }

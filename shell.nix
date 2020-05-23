let
  sources = import ./nix/sources.nix {};

  # build an overlay with specific packages from sources.nix
  overlay = _: pkgs: {
    niv = import sources.niv {};
    # check note bellow
    gopls = pkgs.callPackage ./nix/gopls {};
  };

  # get nixpkgs with pinned packages overlay
  pinnedPkgs = import sources.nixpkgs {
    overlays = [overlay];
  };
in

# This allows overriding pkgs by passing `--arg pkgs ...`
{ pkgs ? pinnedPkgs }:

let
  humanlog = with pkgs; buildGoPackage rec {
    name = "humanlog";
    version = "0.2.1";
    rev = "0.2.1";
    goPackagePath = "github.com/aybabtme/humanlog";
    src = fetchFromGitHub {
      inherit rev;
      owner = "aybabtme";
      repo = "humanlog";
      sha256 = "081r0fw0v0lk75lyvmlq3a8xnz8nyzs0a2090dd1lgj51ki8bm9r";
    };
    meta = with lib; {
      description = "Get human readable logs";
      homepage = "https://github.com/aybabtme/humanlog";
      license = licenses.asl20;
      platforms = platforms.linux;
    };
  };

  go-capataz = import ./default.nix { pkgs = pkgs; };

in
  pkgs.mkShell {
    buildInputs = with pkgs; [
      # bash scripts utilities
      figlet
      stdenv

      # current go version
      go_1_14

      # recommended packages to have for development with emacs/spacemacs
      gotools godef gocode golint golangci-lint gogetdoc gopkgs gotests impl
      errcheck reftools humanlog delve

      # NOTE: I needed to create a gopls package given gotools got broken
      # https://github.com/NixOS/nixpkgs/issues/88716
      gopls

      # capataz deps
      go-capataz
    ];

    shellHook = ''
    unset GOPATH
    unset GOROOT
    '';
  }

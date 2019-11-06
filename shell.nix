let
  # Look here for information about how to generate `nixpkgs-version.json`.
  #  → https://nixos.wiki/wiki/FAQ/Pinning_Nixpkgs
  pinnedVersion = builtins.fromJSON (builtins.readFile ./.nixpkgs-version.json);
  pinnedPkgs = import (builtins.fetchGit {
    inherit (pinnedVersion) url rev;

    ref = "nixos-unstable";
  }) {};

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

in
  pkgs.mkShell {
    buildInputs = with pkgs; [
      # bash scripts utilities
      stdenv
      figlet

      # current go version
      # go_1_12

      # miscellaneous gathering of data
      jq     # json
      miller # csv

      # recommended packages to have for development with emacs/spacemacs
      gotools godef gocode golint golangci-lint gogetdoc gopkgs gotests impl
      errcheck reftools humanlog delve
    ];
  }

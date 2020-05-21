let
  sources = import ./nix/sources.nix {};
  pinnedPkgs = import sources.nixpkgs {};

in

{ pkgs ? pinnedPkgs }:

assert pkgs.lib.versionAtLeast pkgs.go.version "1.14";

pkgs.buildGoPackage rec {
  name = "go-capataz";
  version = "latest";
  goPackagePath = "github.com/capatazlib/go-capataz";
  src = ./.;
  goDeps = ./deps.nix;
}

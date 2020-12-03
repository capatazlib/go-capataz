let
  sources = import ./nix/sources.nix {};
  pinnedPkgs = import sources.nixpkgs {};
in

{ pkgs           ? pinnedPkgs,
  buildGoPackage ? pkgs.buildGoPackage,
  go             ? pkgs.go,
  lib            ? pkgs.lib }:

assert lib.versionAtLeast go.version "1.15";

buildGoPackage rec {
  name = "go-capataz";
  version = "latest";
  goPackagePath = "github.com/capatazlib/go-capataz";
  src = ./.;
  goDeps = ./deps.nix;
}

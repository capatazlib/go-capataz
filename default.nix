let
  sources = import ./nix/sources.nix {};
  pinnedPkgs = import sources.nixpkgs {};
in

{ buildGoPackage ? pinnedPkgs.buildGoPackage,
  go             ? pinnedPkgs.go,
  lib            ? pinnedPkgs.lib }:

assert lib.versionAtLeast go.version "1.14";

buildGoPackage rec {
  name = "go-capataz";
  version = "latest";
  goPackagePath = "github.com/capatazlib/go-capataz";
  src = ./.;
  goDeps = ./deps.nix;
}

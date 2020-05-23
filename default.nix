{ buildGoPackage, go, lib }:

assert lib.versionAtLeast go.version "1.14";

buildGoPackage rec {
  name = "go-capataz";
  version = "latest";
  goPackagePath = "github.com/capatazlib/go-capataz";
  src = ./.;
  goDeps = ./deps.nix;
}

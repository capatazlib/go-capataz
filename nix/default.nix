{ pkgs,
  go,
  buildGoApplication ? pkgs.buildGoApplication,
  lib                ? pkgs.lib }:

assert lib.versionAtLeast go.version "1.17";

buildGoApplication {
  name = "go-capataz";
  version = "latest";
  src = lib.cleanSource ./.;
  modules = ./gomod2nix.toml;
}

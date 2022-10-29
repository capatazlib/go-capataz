_system: _inputs: {
  lib,
  buildGoApplication,
}:
buildGoApplication {
  name = "go-capataz";
  version = "latest";
  src = lib.cleanSource ./.;
  modules = ./gomod2nix.toml;
}

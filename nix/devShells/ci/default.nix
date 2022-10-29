system: {self, ...}: {mkShell}:
mkShell {
  buildInputs = [
    self.packages.${system}.dev-env
  ];

  shellHook = (self.lib.pre-commit system).shellHook;
}

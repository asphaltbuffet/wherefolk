{
  description = "wherefolk - rolodex CLI";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable-small";
    flake-utils.url = "github:numtide/flake-utils";
    nur.url = "github:nix-community/NUR";
  };

  outputs = {
    nixpkgs,
    flake-utils,
    nur,
    ...
  }:
    flake-utils.lib.eachDefaultSystem (
      system: let
        pkgs = import nixpkgs {
          inherit system;
          overlays = [nur.overlays.default];
          config.allowUnfreePredicate = pkg: builtins.elem (pkgs.lib.getName pkg) ["goreleaser-pro"];
        };
      in {
        devShells.default = pkgs.mkShell {
          packages = with pkgs; [
            go
            gopls
            uv
            nodejs
            pkgs.nur.repos.goreleaser.goreleaser-pro
          ];
        };
      }
    );
}

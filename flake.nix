{
  description = "SeamGraph architecture explorer and CLI (GitHub release binaries)";

  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";

  outputs = { nixpkgs, ... }:
    let
      metadata = builtins.fromJSON (builtins.readFile ./nix/release.json);
      release = if metadata == null then
        throw "No SeamGraph release is pinned yet. Publish a release, then run python3 scripts/update-devbox-release.py vX.Y.Z and commit nix/release.json."
      else metadata;
      systems = [ "x86_64-linux" "aarch64-linux" "aarch64-darwin" ];
    in {
      packages = nixpkgs.lib.genAttrs systems (system:
        let
          pkgs = import nixpkgs { inherit system; };
          asset = release.assets.${system};
        in rec {
          seamgraph = pkgs.stdenvNoCC.mkDerivation {
            pname = "seamgraph";
            version = pkgs.lib.removePrefix "v" release.version;
            src = pkgs.fetchurl {
              url = "https://github.com/${release.repository}/releases/download/${release.version}/${asset.name}";
              sha256 = asset.sha256;
            };
            sourceRoot = ".";
            dontConfigure = true;
            dontBuild = true;
            # Preserve release bytes, including the Go linker's macOS signature.
            dontFixup = true;
            installPhase = ''
              runHook preInstall
              install -Dm755 seamgraph "$out/bin/seamgraph"
              runHook postInstall
            '';
            meta = {
              description = "Local architecture analysis and browser explorer";
              homepage = "https://github.com/jani123/seamgraph";
              mainProgram = "seamgraph";
              platforms = systems;
            };
          };
          default = seamgraph;
        });
    };
}

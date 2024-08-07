import argparse
import os
import json
import pathlib
from typing import Any
import urllib


GITHUB_CONTEXT_JSON = os.environ.get("GITHUB_CONTEXT_JSON", "{}")
DOWNLOAD_BASE_URL_TEMPLATE = (
    "https://build.livepeer.live/go-livepeer/{version}/{filename}"
)


def get_embeds(ref_name: str, checksums: list[str]) -> list[dict[Any, Any]]:
    embeds = []
    for line in checksums:
        shasum, filename = line.split()
        download_url = DOWNLOAD_BASE_URL_TEMPLATE.format(
            version=ref_name,
            filename=filename,
        )
        title = filename[9:].split(".")[0]
        parts = title.split("-")
        platform, architecture = parts[0], parts[1]
        if len(parts) == 3:
            architecture = parts[2]
        embeds.append(
            {
                "title": title,
                "url": download_url,
                "color": 2928914,
                "fields": [
                    {"name": "platform", "value": platform, "inline": True},
                    {"name": "architecture", "value": architecture, "inline": True},
                    {"name": "filename", "value": filename, "inline": True},
                    {
                        "name": "Download URL",
                        "value": download_url,
                    },
                    {
                        "name": "SHA256 checksum",
                        "value": f"`{shasum}`",
                    },
                ],
            },
        )
    return embeds


def main(args):
    checksums = []
    github_context = json.loads(GITHUB_CONTEXT_JSON)
    head = github_context.get("head", {})
    user = head.get("user", {})
    repo = head.get("repo", {})
    releases_dir = pathlib.Path("releases/")
    for f in releases_dir.iterdir():
        print(f)
        if f.suffix != ".txt":
            continue
        checksums = f.read_text().splitlines()
    print(checksums)
    content = {
        "content": ":white_check_mark: Build succeeded for go-livepeer :white_check_mark: \n\n"
        f"Branch: `{head.get('ref')}`\n"
        f'Latest commit: "{args.git_commit}" by **{args.git_committer}**\n\n'
        f"Commit SHA: (`{head.get('sha')}`)[{repo.get('commits_url')}]",
        "embed": get_embeds(args.ref_name, checksums),
        "username": user.get("name"),
        "avatar_url": user.get("avatar_url"),
        "flags": 4,
    }
    print(json.dumps(content))


if __name__ == "__main__":
    parser = argparse.ArgumentParser("Discord embed content generator for build system")
    parser.add_argument("--discord-url", help="Discord webhook URL")
    parser.add_argument("--ref-name", help="Tag/branch/commit for current build")
    parser.add_argument("--git-commit", help="git commit message")
    parser.add_argument("--git-committer", help="git commit author name")
    args = parser.parse_args()
    print(args)
    main(args)

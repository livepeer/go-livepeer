import argparse
import os
import json
import pathlib

from discord_webhook import DiscordEmbed, DiscordWebhook


GITHUB_CONTEXT_JSON = os.environ.get("GITHUB_CONTEXT_JSON", "{}")
DOWNLOAD_BASE_URL_TEMPLATE = (
    "https://build.livepeer.live/go-livepeer/{version}/{filename}"
)


def get_embeds(embed: DiscordEmbed, ref_name: str, checksums: list[str]):
    for line in checksums:
        shasum, filename = line.split()
        download_url = DOWNLOAD_BASE_URL_TEMPLATE.format(
            version=ref_name,
            filename=filename,
        )
        title = filename[9:].split(".")[0]
        embed.add_embed_field(name=title, value=download_url, inline=False)
        embed.add_embed_field(name="SHA256 checksum", value=shasum, inline=False)


def main(args):
    checksums = []
    github_context = json.loads(GITHUB_CONTEXT_JSON)
    head = github_context.get("head", {})
    repo = head.get("repo", {})
    checksums_file = pathlib.Path("releases") / f"{args.ref_name}_checksums.txt"
    commit_url = f'{repo.get("html_url")}/commit/{head.get("sha")}'
    checksums = checksums_file.read_text().splitlines()
    webhook = DiscordWebhook(
        url=args.discord_url,
        content=":white_check_mark: Build succeeded for go-livepeer :white_check_mark:",
        username="[BOT] Livepeer builder",
    )
    webhook.add_file(filename=checksums_file.name, file=checksums_file.read_bytes())
    embed = DiscordEmbed(
        title=head.get("ref"),
        description=args.git_commit,
        color=2928914,
        url=commit_url,
    )
    embed.add_embed_field(name="Commit SHA", value=head.get("sha"))
    embed.set_author(name=args.git_committer)
    get_embeds(embed, args.ref_name, checksums)
    embed.set_timestamp()
    webhook.add_embed(embed)
    response = webhook.execute()
    print(response)


if __name__ == "__main__":
    parser = argparse.ArgumentParser("Discord embed content generator for build system")
    parser.add_argument("--discord-url", help="Discord webhook URL")
    parser.add_argument("--ref-name", help="Tag/branch/commit for current build")
    parser.add_argument("--git-commit", help="git commit message")
    parser.add_argument("--git-committer", help="git commit author name")
    args = parser.parse_args()
    main(args)

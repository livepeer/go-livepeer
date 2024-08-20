import argparse
import os
import json
import pathlib
from typing import Any

from requests import Response
from discord_webhook import DiscordEmbed, DiscordWebhook


GITHUB_CONTEXT_JSON = os.environ.get("GITHUB_CONTEXT_JSON", "{}")
DOWNLOAD_BASE_URL_TEMPLATE = (
    "https://build.livepeer.live/go-livepeer/{version}/{filename}"
)


def get_github_context_vars(context: dict[str, Any]) -> dict[str, str]:
    context_vars = {}
    event: dict[str, Any] = context["event"]
    if context.get("event_name") == "pull_request":
        pull_request_context: dict[str, Any] = event["pull_request"]
        head = pull_request_context.get("head", {})
        repo = head.get("repo", {})
        sha = head.get("sha")
        context_vars["title"] = head.get("ref")
        context_vars["sha"] = sha
        context_vars["commit_url"] = f'{repo.get("html_url")}/commit/{sha}'
    elif context.get("event_name") == "push":
        sha = event["after"]
        context_vars["title"] = context["ref_name"]
        context_vars["sha"] = sha
        context_vars["commit_url"] = event["compare"]
    return context_vars


def populate_embeds(embed: DiscordEmbed, ref_name: str, checksums: list[str]):
    for line in checksums:
        _, filename = line.split()
        download_url = DOWNLOAD_BASE_URL_TEMPLATE.format(
            version=ref_name,
            filename=filename,
        )
        title = filename.removeprefix("livepeer-").split(".")[0]
        print(f"Adding embed field name={title} value={download_url}")
        embed.add_embed_field(name=title, value=download_url, inline=False)


def main(args):
    checksums = []
    github_context: dict[str, Any] = json.loads(GITHUB_CONTEXT_JSON)
    context_vars = get_github_context_vars(github_context)
    checksums_file = pathlib.Path("releases") / f"{args.ref_name}_checksums.txt"
    checksums = checksums_file.read_text().splitlines()
    webhook = DiscordWebhook(
        url=args.discord_url,
        content=":white_check_mark: Build succeeded for go-livepeer :white_check_mark:",
        username="[BOT] Livepeer builder",
    )
    webhook.add_file(filename=checksums_file.name, file=checksums_file.read_bytes())
    embed = DiscordEmbed(
        title=context_vars.get("title"),
        description=args.git_commit,
        color=2928914,
        url=context_vars.get("commit_url"),
    )
    embed.add_embed_field(name="Commit SHA", value=context_vars.get("sha"))
    embed.set_author(name=args.git_committer)
    populate_embeds(embed, args.ref_name, checksums)
    embed.set_timestamp()
    webhook.add_embed(embed)
    response: Response = webhook.execute()
    print("sending webhook with content:")
    print(webhook.json)
    # Fail the script if discord returns anything except OK status
    assert (
        response.ok
    ), f"Discord webhook failed {response.status_code} {response.content}"


if __name__ == "__main__":
    parser = argparse.ArgumentParser("Discord embed content generator for build system")
    parser.add_argument("--discord-url", help="Discord webhook URL")
    parser.add_argument("--ref-name", help="Tag/branch/commit for current build")
    parser.add_argument("--git-commit", help="git commit message")
    parser.add_argument("--git-committer", help="git commit author name")
    args = parser.parse_args()
    main(args)

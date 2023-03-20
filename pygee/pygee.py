import logging

import click
import click_logging

from enkit.pygee import version

LOG = logging.getLogger(__name__)
click_logging.basic_config(LOG)


@click.group(
    commands=[
        version.root,
    ],
)
@click_logging.simple_verbosity_option(LOG)
@click.pass_context
def root(context):
    pass


if __name__ == "__main__":
    root()

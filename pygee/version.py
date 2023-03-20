import logging
import sys

import click
import click_logging

LOG = logging.getLogger(__name__)
click_logging.basic_config(LOG)


@click_logging.simple_verbosity_option(LOG)
@click.command(name="version")
def root():
    click.secho("version is not yet implemented", err=True)
    sys.exit(1)

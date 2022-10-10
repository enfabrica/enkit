#!/usr/bin/python3

import argparse
import logging
import os
import os.path
import re
import subprocess
import tempfile

re_key_equals_val = re.compile(r'^ *(\S+) = "(\S*)"$')
re_dict_entry = re.compile(r'^ *"(\S+)": (\S.*?),?$')

class AstoreHandler:
    def __init__(self):
        self.text = ''  # text of uidfile

    def parse_arguments(self):
        parser = argparse.ArgumentParser()
        parser.add_argument('--uidfile', type=str, help='The BUILD file to edit.', required=True)
        parser.add_argument('--astore_file', type=str, help='The filename to use to save the file to astore', required=False)
        parser.add_argument('--astore_dir', type=str, help='The directory to upload the targets to.', required=False)
        parser.add_argument('--target', type=str, action='append', default=[], help='The targets to upload', required=True)
        parser.add_argument('--astore_bin', type=str, help='Override path to the astore binary.', required=False)
        parser.add_argument('--label', type=str, default=None, help='Label to use identifying this target')

        self.args = parser.parse_args()
        self.astore_bin = self.args.astore_bin
        return self.args

    def astore_upload(self, target):
        temp_fd, temp_filename = tempfile.mkstemp(suffix='.toml', prefix='astore.')
        os.close(temp_fd)

        cproc = subprocess.run(['sha256sum', target], check=True, capture_output=True)
        file_sha = cproc.stdout.decode('utf-8').split()[0]

        if self.args.astore_file:
            filename = self.args.astore_file
        elif self.args.astore_dir:
            filename = self.args.astore_dir + "/" + os.path.basename(target)
        cmd = [self.astore_bin, 'upload', '-G', '-f', filename, target, '-m', temp_filename]

        print(cmd)
        subprocess.run(cmd, check=True)

        results = dict()
        with open(temp_filename, 'r') as fd:
            for line in fd:
                mo = re_key_equals_val.match(line)
                if mo:
                    results[mo.group(1)] = mo.group(2)
        file_uid = results['Uid']
        os.remove(temp_filename)

        return file_uid, file_sha

    def update_uidfile(self, label, file_uid, file_sha):
        varname = label.upper()
        varname = re.sub(r'\W+', '_', varname)

        if self.text:
            blocks = self.text.split('\n\n')
        else:
            blocks = [
                        '# Do not manually edit this file.',
                        f"SHA_{varname} = \"foo\"",
                        "MAP = {\n}"
                        ]

        # find a UID_FOO = "something" block, and either replace or insert
        # our variables:
        found = False
        for i in range(len(blocks)):
            if blocks[i].startswith('SHA_') or blocks[i].startswith('UID_'):
                found = True
                kv_pairs = dict()
                for line in blocks[i].splitlines(keepends = False):
                    mo = re_key_equals_val.match(line)
                    if mo:
                        kv_pairs[mo.group(1)] = mo.group(2)
                    else:
                        logging.warn(f"Unparseable: {line!r}")
                kv_pairs[f'SHA_{varname}'] = file_sha
                kv_pairs[f'UID_{varname}'] = file_uid
                blocks[i] = '\n'.join([f"{k} = \"{kv_pairs[k]}\"" for k in sorted(kv_pairs.keys())])
                break
        if not found:
            new_block = f"SHA_{varname} = \"{file_sha}\"\nUID_{varname} = \"{file_uid}\""
            blocks.insert(0, new_block)

        # if a MAP = {...} block exists, and either insert or replace our keys.
        for i in range(len(blocks)):
            if blocks[i].startswith('MAP = {'):
                lines = list(blocks[i].splitlines())
                lines = lines[1:]
                lines = lines[:-1]
                kv_pairs = dict()
                for line in lines:
                    mo = re_dict_entry.match(line)
                    if mo:
                        kv_pairs[mo.group(1)] = mo.group(2)
                    else:
                        logging.warn(f"Unparseable: {line!r}")
                kv_pairs[label] = f'[UID_{varname}, SHA_{varname}]'
                blocks[i] = 'MAP = {\n' + '\n'.join([f"  \"{k}\": {kv_pairs[k]}," for k in sorted(kv_pairs.keys())]) + '\n}'
                break

        output = '\n\n'.join(blocks)
        if not output.endswith('\n'):
            output += '\n'
        self.text = output
        return output

    def read_uid_file(self):
        if not os.path.isfile(self.args.uidfile):
            logging.warn('Uidfile %r not found, will be created.', self.args.uidfile)
            return

        with open(self.args.uidfile, 'r', encoding='utf-8') as fd:
            self.text = fd.read()

    def write_uid_file(self):
        with open(self.args.uidfile, 'w', encoding='utf-8') as fd:
            fd.write(self.text)

    def process_target(self, target):
        file_uid, file_sha = self.astore_upload(target)
        label = self.args.label
        if not label:
            label = os.path.basename(target)
        self.update_uidfile(label, file_uid, file_sha)

    def process_all_targets(self):
        for target in self.args.target:
            self.process_target(target)


def main():
    h = AstoreHandler()
    h.parse_arguments()
    h.read_uid_file()
    h.process_all_targets()
    h.write_uid_file()

if __name__ == '__main__':
    main()

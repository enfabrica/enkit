import paramiko.ssh_exception
from absl import app
from typing import Optional, List, Tuple, Dict
import os
import re
import argparse
import paramiko
import time
import socket
import json
from threading import Thread
from pathlib import Path
import logging
from dataclasses import dataclass, field

  
@dataclass
class GpuInfo:
    bus_id: str
    gpu_model: str
    card_model: str


@dataclass
class HostInfo:
    hostname: str
    reachable: bool = False
    clientd_healthy: bool = False
    clientd_fuse_ok: bool = False
    gpu_infos: List[GpuInfo] = field(default_factory=list)

    def __repr__(self):
        return f"Host[{self.hostname}]<clientd_healthy: {self.clientd_healthy}>"


class HostSession(Thread):
    def __init__(self, hostname: str, *args, **kwargs):
        super().__init__(*args, **kwargs)
        self.hostname = hostname
        self.host_info = HostInfo(hostname)
        self.daemon = True

        self._client: Optional[paramiko.SSHClient] = None
        self._client_created_at: Optional[float] = None

    @classmethod
    def for_hostname(cls, hostname: str) -> "HostSession":
        session = cls(hostname)
        session.start()
        return session
    
    @staticmethod
    def _get_ssh_config() -> paramiko.SSHConfig:
        config_path = Path.home() / ".ssh" / "config"
        if not config_path.exists():
            logging.error("You must run 'enkit machine upgrade' first")
            raise RuntimeError("Machine not fully configured with enkit")
        
        return paramiko.SSHConfig.from_path(config_path)
    
    def _get_proxy_command(self) -> paramiko.ProxyCommand:
        ssh_config = self._get_ssh_config()
        host_config = ssh_config.lookup(self.hostname)

        if 'proxycommand' not in host_config:
            logging.error(f"ProxyCommand missing for '{self.hostname}' in ssh config")
            raise RuntimeError(f"Machine not configured for tunnel to '{self.hostname}'")

        return paramiko.ProxyCommand(host_config["proxycommand"])

    @property
    def client(self) -> paramiko.SSHClient:
        # recreate after some period of time to ensure a health connection
        if self._client_created_at is not None and (time.time() - self._client_created_at) > 600:
            self._client = self._client_created_at = None

        if self._client is None:
            self._client = paramiko.SSHClient()
            self._client.set_missing_host_key_policy(paramiko.AutoAddPolicy())
            self._client_created_at = time.time()

            # weird workaround for bug in paramiko handling of certain key types
            pkey = paramiko.Agent().get_keys()[0]
            if not hasattr(pkey, "public_blob"):
                pkey.public_blob = None

            self._client.connect(self.hostname, sock=self._get_proxy_command(), allow_agent=False, pkey=pkey, timeout = 10)
        
        return self._client
    
    def _run_command(self, cmd: str) -> Tuple[int, str, str]:  # returncode, stdout, stderr
        _, stdout, stderr = self.client.exec_command(cmd)

        # wait for command to finish
        returncode = stdout.channel.recv_exit_status()

        return returncode, stdout.read().decode(), stderr.read().decode()

    def _is_reachable(self) -> bool:
        try:
            transport = self.client.get_transport()
            if transport is not None and transport.is_active():
                logging.debug(f"Host '{self.hostname}' is reachable")
                return True
            return False
        except (socket.gaierror, paramiko.ssh_exception.SSHException, TimeoutError) as e:
            logging.error(f"Cannot reach '{self.hostname}': {e}")
            return False
        
    def _is_clientd_healthy(self) -> bool:
        if self.host_info.reachable:
            rc, _, _ = self._run_command("curl localhost:9981/-/healthy")
            return rc == 0
        return False
    
    def _is_clientd_fuse_ok(self) -> bool:
        if self.host_info.reachable:
            rc, _, _ = self._run_command("ls /opt/enf_mounts/buildbarn/")
            return rc == 0
        return False
    
    def _get_gpu_infos(self) -> List[GpuInfo]:
        infos: List[GpuInfo] = []

        if self.host_info.reachable:
            rc, stdout, _ = self._run_command("lspci | grep -i nvidia")
            for line in stdout.splitlines():
                line = line.strip()
                match = re.match(r"(\d\d\:\d\d\.\d).+\:(.+)\[(.+)\]", line)
                if match:
                    bus_id = match.group(1).strip()
                    gpu_model = match.group(2).strip()
                    card_model = match.group(3).strip()
                    infos.append(GpuInfo(bus_id, gpu_model, card_model))

        return infos

    def run(self):
        logging.info(f"Starting session for '{self.hostname}'")
        try:            
            self.host_info.reachable = self._is_reachable()
            self.host_info.clientd_healthy = self._is_clientd_healthy()                    
            self.host_info.clientd_fuse_ok = self._is_clientd_fuse_ok()
            self.host_info.gpu_infos.extend(self._get_gpu_infos())
        except Exception as e:
            logging.error(f"Unhandled Exception: {e}")
        finally:
            if self._client:
                self._client.close()

    def join(self, timeout = None):
        super().join(timeout)
        

class SessionPool:
    MAX_CONCURRENT_SESSIONS = 20

    def __init__(self):
        self._sessions: List[HostSession] = []
        self.reachable_hosts: List[HostInfo] = []

    def _wait_until_free(self):
        while True:                    
            for session in self._sessions:
                if not session.is_alive():
                    session.join()
                    if session.host_info.reachable:
                        self.reachable_hosts.append(session.host_info)
                    self._sessions.remove(session)
                    
            if len(self._sessions) < self.MAX_CONCURRENT_SESSIONS:
                return

    def add_for_hostname(self, hostname: str):
        self._wait_until_free()
        self._sessions.append(HostSession.for_hostname(hostname))

    def wait_for_all(self):
        for session in self._sessions:
            session.join()
            if session.host_info.reachable:
                self.reachable_hosts.append(session.host_info)

    def print(self):
        logging.info("Reachable Hosts")
        for host_info in self.reachable_hosts:
            logging.info(f"  {host_info}")

    def write_to_json(self, path: Path):
        sorted_hosts = sorted(self.reachable_hosts, key=lambda x: x.hostname)
        out_json = {
            "hosts": [host_info.__dict__ for host_info in sorted_hosts]
        }
        with path.open("w") as f:
            json.dump(out_json, f, indent=2, default=lambda o: o.__dict__)


potential_hosts = [f"nc-gpu-{i:02}.rdu" for i in range(1, 40)]
    

def main(argv):
    parser = argparse.ArgumentParser()
    parser.add_argument("--json", type=Path, help="Optional path to file to which to write host info in JSON format")
    parser.add_argument("--quiet", default=False, action="store_true", help="Do not write host info to stdout")
    args = parser.parse_args(argv[1:])

    session_pool = SessionPool()

    # check all possible nc-gpu-NN addresses
    for hostname in potential_hosts:
        session_pool.add_for_hostname(hostname)

    session_pool.wait_for_all()

    json_out_path = args.json or os.environ.get("JSON_OUT_PATH")
    json_out_path = Path(json_out_path).absolute() if json_out_path else None

    if not args.quiet:
        session_pool.print()

    if json_out_path:
        logging.info(f"Writing output to: {json_out_path}")
        session_pool.write_to_json(json_out_path)
   

if __name__ == "__main__":
    app.run(main)

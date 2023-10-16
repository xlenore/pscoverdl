import os
import re
import concurrent.futures
import yaml
import json
from termcolor import colored
from tqdm import tqdm
from pathlib import Path
import requests

PS1_COVERS_URL_DEFAULT = (
    "https://raw.githubusercontent.com/xlenore/psx-covers/main/covers/default"
)
PS1_COVERS_URL_3D = (
    "https://raw.githubusercontent.com/xlenore/psx-covers/main/covers/3d"
)

PS2_COVERS_URL_DEFAULT = (
    "https://raw.githubusercontent.com/xlenore/ps2-covers/main/covers/default"
)
PS2_COVERS_URL_3D = (
    "https://raw.githubusercontent.com/xlenore/ps2-covers/main/covers/3d"
)


class BaseCoverDownloader:
    def __init__(self, cover_dir, gamelist_dir, cover_type, use_ssl, emulator):
        self.cover_dir = Path(cover_dir)
        self.gamelist_dir = gamelist_dir
        self.cover_type = cover_type
        self.use_ssl = use_ssl
        self.emulator = emulator

    def get_serial_list(self, gamelist_cache_path, existing_covers):
        if not os.path.exists(gamelist_cache_path):
            print(colored("[ERROR]: gamelist.cache file not found", "red"))
            return []

        with open(gamelist_cache_path, errors="ignore") as file:
            regex = re.findall(r"(\w{4}-\d{5})", file.read())
            serial_list = list(set(regex))
            print(colored(f"[LOG]: {len(serial_list)} games found", "green"))
            print(colored(f"[LOG]: Removing already downloaded covers...", "green"))
            serial_list = [
                game_serial
                for game_serial in serial_list
                if game_serial not in existing_covers
            ]

            return serial_list

    def existing_covers(self):
        covers = [
            filename.stem
            for filename in self.cover_dir.glob("*.jpg")
            if filename.suffix in [".jpg", ".png"]
        ]
        return covers

    def serial_to_name(self, name_list, game_serial):
        return name_list.get(game_serial)

    def download_cover(self, url, cover_path):
        try:
            if not self.use_ssl:
                url = url.replace("https://", "http://")
            response = requests.get(url)
            if response.status_code == 200:
                with open(cover_path, "wb") as file:
                    file.write(response.content)
                return True
        except requests.exceptions.RequestException:
            pass
        return False

    def download(self):
        if not self.cover_dir.exists():
            self.cover_dir.mkdir(parents=True)

        existing_covers = self.existing_covers()
        name_list = self.get_name_list()
        serial_list = self.get_serial_list(self.gamelist_dir, existing_covers)

        if self.emulator == "pcsx2":
            covers_url_default = PS2_COVERS_URL_DEFAULT
            covers_url_3d = PS2_COVERS_URL_3D
        elif self.emulator == "duckstation":
            covers_url_default = PS1_COVERS_URL_DEFAULT
            covers_url_3d = PS1_COVERS_URL_3D
        else:
            print(colored(f"[ERROR]: Invalid emulator: {self.emulator}", "red"))
            return

        covers_url = covers_url_default
        if self.cover_type == 1:
            covers_url = covers_url_3d

        print(covers_url)

        if self.cover_type == 0:
            cover_urls = [
                f"{covers_url}/{game_serial}.jpg"
                for game_serial in serial_list
                if game_serial not in existing_covers
            ]
        elif self.cover_type == 1:
            cover_urls = [
                f"{covers_url}/{game_serial}.png"
                for game_serial in serial_list
                if game_serial not in existing_covers
            ]

        if not serial_list:
            print(colored(f"[LOG]: All covers have already been downloaded", "green"))
            return

        workers = 4
        with concurrent.futures.ThreadPoolExecutor(max_workers=workers) as executor:
            results = []
            for url in cover_urls:
                cover_path = self.cover_dir.joinpath(Path(url).name)
                results.append(executor.submit(self.download_cover, url, cover_path))

            for result, url in tqdm(
                zip(results, cover_urls),
                total=len(cover_urls),
                desc="Downloading covers",
                unit="cover",
                ncols=50,
                bar_format="{l_bar}{bar}| {n_fmt}/{total_fmt}",
            ):
                game_serial = Path(url).stem
                game_name = self.serial_to_name(name_list, game_serial)

                if result.result():
                    tqdm.write(colored(f"{game_serial} | {game_name}", "green"))
                else:
                    tqdm.write(
                        colored(
                            f"[{game_serial} | {game_name}] not found. Skipping...",
                            "yellow",
                        )
                    )


class PCSX2CoverDownloader(BaseCoverDownloader):
    def __init__(self, cover_dir, gamelist_dir, cover_type, use_ssl, emulator):
        super().__init__(cover_dir, gamelist_dir, cover_type, use_ssl, emulator)

    def get_name_list(self):
        name_list = {}

        gameindex_file = (
            Path(__file__).resolve().parent.joinpath("resources", "GameIndex.yaml")
        )

        if not gameindex_file.exists():
            print(colored("[ERROR]: GameIndex.yaml file not found", "red"))
            return {}

        with open(gameindex_file, encoding="utf-8-sig") as file:
            name_list = {
                key: value["name"]
                for key, value in yaml.load(file, Loader=yaml.CBaseLoader).items()
            }
        return name_list


class DuckStationCoverDownloader(BaseCoverDownloader):
    def __init__(self, cover_dir, gamelist_dir, cover_type, use_ssl, emulator):
        super().__init__(cover_dir, gamelist_dir, cover_type, use_ssl, emulator)

    def get_name_list(self):
        name_list = {}

        gamedb_file = (
            Path(__file__).resolve().parent.joinpath("resources", "gamedb.json")
        )

        if not gamedb_file.exists():
            print(colored("[ERROR]: gamedb.json file not found", "red"))
            return {}

        with open(gamedb_file, encoding="utf-8") as file:
            gameindex = json.load(file)
            name_list = {item["serial"]: item["name"] for item in gameindex}
        return name_list


def download_covers(cover_dir, gamelist_dir, cover_type, use_ssl, emulator):
    if emulator == "pcsx2":
        downloader = PCSX2CoverDownloader(
            cover_dir, gamelist_dir, cover_type, use_ssl, emulator
        )
    elif emulator == "duckstation":
        downloader = DuckStationCoverDownloader(
            cover_dir, gamelist_dir, cover_type, use_ssl, emulator
        )
    else:
        print(colored(f"[ERROR]: Invalid emulator: {emulator}", "red"))
        return

    downloader.download()

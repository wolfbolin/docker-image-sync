# encoding=utf-8
import json
import re
from repository import *


def main():
    image_name = "alpine"
    tag_regex = r"^.\..*$"

    tags = []
    for tag in docker_image_tags(image_name):
        if re.match(tag_regex, tag["name"]):
            tags.append(tag['name'])

    print(tags)


if __name__ == '__main__':
    main()

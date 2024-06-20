FROM hub.wiolfi.net/python:3.12
LABEL authors="wolfbolin"

WORKDIR /opt/app
COPY Pipfile.lock /opt/app
RUN apt update && apt upgrade -y && apt install -y skopeo \
    && pip install -i https://pypi.tuna.tsinghua.edu.cn/simple pipenv \
    && pipenv sync

CMD pipenv run python main.py
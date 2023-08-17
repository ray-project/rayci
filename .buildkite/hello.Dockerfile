FROM cr.ray.io/rayproject/forge

COPY .buildkite/hello.txt /opt/app/hello.txt

CMD ["echo", "hello world"]

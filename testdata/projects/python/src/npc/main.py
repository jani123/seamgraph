from world.api import Reader, open_world as make_world

def read_world() -> str:
    reader: Reader = make_world()
    return reader.read()

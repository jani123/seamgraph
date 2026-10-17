class Reader:
    def read(self) -> str:
        return 'world'

def open_world() -> Reader:
    return Reader()

def secret() -> str:
    return 'secret'

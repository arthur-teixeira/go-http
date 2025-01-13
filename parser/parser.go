package parser

const (
  OPTIONS = "OPTIONS"
  GET = "GET"
  HEAD = "HEAD"
  POST = "POST"
  PUT = "PUT"
  DELETE = "DELETE"
  TRACE = "TRACE"
  CONNECT = "CONNECT"
)

type Request struct {
  Method string
  URI string
  Version string
}

type RequestParser struct {
}


func (p *RequestParser) Parse(request string) (*Request, error) {
  return nil, nil
}

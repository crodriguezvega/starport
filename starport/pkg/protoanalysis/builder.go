package protoanalysis

import (
	"regexp"
	"strings"

	"github.com/emicklei/proto"
)

type builder struct {
	p pkg
}

// buil turns a low lovel proto pkg into a high level Package.
func build(p pkg) Package {
	br := builder{p}

	pk := Package{
		Name:     p.name,
		Path:     p.dir,
		Files:    br.buildFiles(),
		Messages: br.buildMessages(),
		Services: br.toServices(p.services()),
	}

	for _, option := range p.options() {
		if option.Name == optionGoPkg {
			pk.GoImportName = option.Constant.Source
			break
		}
	}

	return pk
}

func (b builder) buildFiles() (files []File) {
	for _, f := range b.p.files {
		files = append(files, File{f.path, f.imports})
	}

	return
}

func (b builder) buildMessages() (messages []Message) {
	for _, f := range b.p.files {
		for _, message := range f.messages {
			messages = append(messages, Message{
				Name: message.Name,
				Path: f.path,
			})
		}
	}

	return
}

func (b builder) toServices(ps []*proto.Service) (services []Service) {
	for _, service := range ps {
		s := Service{
			Name:     service.Name,
			RPCFuncs: b.elementsToRPCFunc(service.Elements),
		}

		services = append(services, s)
	}

	return
}

func (b builder) elementsToRPCFunc(elems []proto.Visitee) (rpcFuncs []RPCFunc) {
	for _, el := range elems {
		rpc, ok := el.(*proto.RPC)
		if !ok {
			continue
		}

		var requestMessage *proto.Message

		for _, message := range b.p.messages() {
			if message.Name != rpc.RequestType {
				continue
			}
			requestMessage = message
		}

		if requestMessage == nil {
			continue
		}

		rf := RPCFunc{
			Name:        rpc.Name,
			RequestType: rpc.RequestType,
			ReturnsType: rpc.ReturnsType,
			HTTPRules:   b.elementsToHTTPRules(requestMessage, rpc.Elements),
		}

		rpcFuncs = append(rpcFuncs, rf)
	}

	return rpcFuncs
}

func (b builder) elementsToHTTPRules(requestMessage *proto.Message, elems []proto.Visitee) (httpRules []HTTPRule) {
	for _, el := range elems {
		option, ok := el.(*proto.Option)
		if !ok {
			continue
		}
		if !strings.Contains(option.Name, "google.api.http") {
			continue
		}

		httpRules = append(httpRules, b.constantToHTTPRules(requestMessage, option.Constant)...)
	}

	return
}

var urlParamRe = regexp.MustCompile(`(?m){(.+?)}`)

func (b builder) constantToHTTPRules(requestMessage *proto.Message, constant proto.Literal) (httpRules []HTTPRule) {
	// find out the endpoint template.
	endpoint := constant.Source

	if endpoint == "" {
		for key, val := range constant.Map {
			switch key {
			case
				"get",
				"post",
				"put",
				"patch",
				"delete":
				endpoint = val.Source
			}
			if endpoint != "" {
				break
			}
		}
	}

	// find out url params.
	var params []string

	match := urlParamRe.FindAllStringSubmatch(endpoint, -1)
	for _, item := range match {
		params = append(params, item[1])
	}

	// calculate url params, query params and body fields counts.
	var (
		messageFieldsCount = b.messageFieldsCount(requestMessage)
		paramsCount        = len(params)
		bodyFieldsCount    int
	)

	if body, ok := constant.Map["body"]; ok { // check if body is specified.
		if body.Source == "*" { // means there should be no query params per the spec.
			bodyFieldsCount = messageFieldsCount - paramsCount
		} else if body.Source != "" {
			bodyFieldsCount = 1 // means body fields are grouped under a single top-level field.
		}
	}

	queryParamsCount := messageFieldsCount - paramsCount - bodyFieldsCount

	// create and add the HTTP rule to the list.
	httpRule := HTTPRule{
		Params:   params,
		HasQuery: queryParamsCount > 0,
		HasBody:  bodyFieldsCount > 0,
	}

	httpRules = append(httpRules, httpRule)

	// search for nested HTTP rules.
	if constant, ok := constant.Map["additional_bindings"]; ok {
		httpRules = append(httpRules, b.constantToHTTPRules(requestMessage, *constant)...)
	}

	return httpRules
}

func (b builder) messageFieldsCount(message *proto.Message) (count int) {
	for _, el := range message.Elements {
		switch el.(type) {
		case
			*proto.NormalField,
			*proto.MapField,
			*proto.OneOfField:
			count++
		}
	}

	return
}

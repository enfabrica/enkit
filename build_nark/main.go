// Copyright 2020 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
// This file has been modified by Enfabrica Corp. from the version obtained from
// the above copyright holder.

package main

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"mime/quotedprintable"
	"os"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/GoogleCloudPlatform/cloud-build-notifiers/lib/notifiers"
	"github.com/golang/protobuf/proto"
	cbpb "google.golang.org/genproto/googleapis/devtools/cloudbuild/v1"
	"gopkg.in/gomail.v2"

	"github.com/enfabrica/enkit/lib/logger"
)

const (
	contentType = "text/html"
)

var log = logger.DefaultLogger{
	Printer: logrus.Printf,
}

func main() {
	log.Infof("starting email notifier")
	if err := notifiers.Main(new(smtpNotifier)); err != nil {
		log.Errorf("fatal error: %v", err)
		os.Exit(1)
	}
}

type smtpNotifier struct {
	filter   notifiers.EventFilter
	tmpl     *template.Template
	mcfg     mailConfig
	br       notifiers.BindingResolver
	tmplView *notifiers.TemplateView
}

type mailConfig struct {
	server, sender, from, password, localName string
	port                                      int
	recipients                                []string
}

func (s *smtpNotifier) SetUp(ctx context.Context, cfg *notifiers.Config, cfgTemplate string, sg notifiers.SecretGetter, br notifiers.BindingResolver) error {
	prd, err := notifiers.MakeCELPredicate(cfg.Spec.Notification.Filter)
	if err != nil {
		return fmt.Errorf("failed to create CELPredicate: %w", err)
	}
	s.filter = prd
	tmpl, err := template.New("email_template").Parse(cfgTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse HTML email template: %w", err)
	}
	s.tmpl = tmpl

	mcfg, err := getMailConfig(ctx, sg, cfg.Spec)
	if err != nil {
		return fmt.Errorf("failed to construct a mail delivery config: %w", err)
	}
	s.mcfg = mcfg
	s.br = br
	return nil
}

func getMailConfig(ctx context.Context, sg notifiers.SecretGetter, spec *notifiers.Spec) (mailConfig, error) {
	delivery := spec.Notification.Delivery

	server, ok := delivery["server"].(string)
	if !ok {
		return mailConfig{}, fmt.Errorf("expected delivery config %v to have string field `server`", delivery)
	}
	log.Infof("Server: %s", server)
	port, ok := delivery["port"].(int)
	if !ok {
		return mailConfig{}, fmt.Errorf("expected delivery config %v to have string field `port`", delivery)
	}
	log.Infof("Port: %d", port)
	sender, ok := delivery["sender"].(string)
	if !ok {
		return mailConfig{}, fmt.Errorf("expected delivery config %v to have string field `sender`", delivery)
	}

	localName, ok := delivery["localName"].(string)
	if !ok {
		return mailConfig{}, fmt.Errorf("expected delivery config %v to have string field `localName`", delivery)
	}
	log.Infof("LocalName: %s", localName)
	from, ok := delivery["from"].(string)
	if !ok {
		return mailConfig{}, fmt.Errorf("expected delivery config %v to have string field `from`", delivery)
	}
	log.Infof("From: %s", from)
	ris, ok := delivery["recipients"].([]interface{})
	if !ok {
		return mailConfig{}, fmt.Errorf("expected delivery config %v to have repeated field `recipients`", delivery)
	}
	recipients := make([]string, 0, len(ris))
	for _, ri := range ris {
		r, ok := ri.(string)
		if !ok {
			return mailConfig{}, fmt.Errorf("failed to convert recipient (%v) into a string", ri)
		}
		recipients = append(recipients, r)
	}
	log.Infof("Recipients: %s", strings.Join(recipients, ", "))
	passwordRef, err := notifiers.GetSecretRef(delivery, "password")
	if err != nil {
		return mailConfig{}, fmt.Errorf("failed to get ref for secret field `password`: %w", err)
	}

	passwordResource, err := notifiers.FindSecretResourceName(spec.Secrets, passwordRef)
	if err != nil {
		return mailConfig{}, fmt.Errorf("failed to find Secret resource name for reference %q: %w", passwordRef, err)
	}

	password, err := sg.GetSecret(ctx, passwordResource)
	if err != nil {
		return mailConfig{}, fmt.Errorf("failed to get SMTP password: %w", err)
	}

	return mailConfig{
		server:     server,
		port:       port,
		sender:     sender,
		localName:  localName,
		from:       from,
		password:   password,
		recipients: recipients,
	}, nil
}

func (s *smtpNotifier) SendNotification(ctx context.Context, build *cbpb.Build) error {
	if !s.filter.Apply(ctx, build) {
		log.Infof("no mail for event:\n%s", proto.MarshalTextString(build))
		return nil
	}

	bindings, err := s.br.Resolve(ctx, nil, build)
	if err != nil {
		log.Errorf("failed to resolve bindings :%v", err)
	}
	s.tmplView = &notifiers.TemplateView{
		Build:  &notifiers.BuildView{Build: build},
		Params: bindings,
	}
	log.Infof("sending email for (build id = %q, status = %s)", build.GetId(), build.GetStatus())
	return s.sendSMTPNotification()
}

func (s *smtpNotifier) sendSMTPNotification() error {
	msg, err := s.buildEmail()
	if err != nil {
		log.Warnf("failed to build email: %v", err)
		return fmt.Errorf("failed to build email: %w", err)
	}

	d := gomail.NewDialer(s.mcfg.server, s.mcfg.port, s.mcfg.sender, s.mcfg.password)
	d.LocalName = s.mcfg.localName
	sc, err := d.Dial()
	if err != nil {
		return fmt.Errorf("failed to dial: %w", err)
	}
	defer sc.Close()

	if err = gomail.Send(sc, msg); err != nil {
		return fmt.Errorf("failed to send email: %w", err)
	}

	log.Infof("email sent successfully")
	return nil
}

func (s *smtpNotifier) buildEmail() (*gomail.Message, error) {
	msg := gomail.NewMessage()
	build := s.tmplView.Build
	logURL, err := notifiers.AddUTMParams(s.tmplView.Build.LogUrl, notifiers.EmailMedium)
	if err != nil {
		return nil, fmt.Errorf("failed to add UTM params: %w", err)
	}
	build.LogUrl = logURL

	body := new(bytes.Buffer)
	if err := s.tmpl.Execute(body, s.tmplView); err != nil {
		return nil, err
	}

	msg.SetBody(fmt.Sprintf(`%s; charset="utf-8"`, contentType), body.String())

	subject := fmt.Sprintf("Cloud Build [%s]: %s", build.ProjectId, build.Id)
	msg.SetHeader("Subject", subject)

	if s.mcfg.from != s.mcfg.sender {
		msg.SetHeader("Sender", s.mcfg.sender)
	}

	msg.SetHeader("From", s.mcfg.from)
	msg.SetHeader("To", s.mcfg.recipients...)
	msg.SetHeader("MIME-Version", "1.0")
	msg.SetHeader("Content-Type", fmt.Sprintf(`%s; charset="utf-8"`, contentType))
	msg.SetHeader("Content-Transfer-Encoding", "quoted-printable")
	msg.SetHeader("Content-Disposition", "inline")

	encoded := new(bytes.Buffer)
	finalMsg := quotedprintable.NewWriter(encoded)
	finalMsg.Write(body.Bytes())
	if err := finalMsg.Close(); err != nil {
		return nil, fmt.Errorf("failed to close MIME writer: %w", err)
	}

	return msg, nil
}
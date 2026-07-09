# frozen_string_literal: true

# XSLT 1.0 transforms via Nokogiri::XSLT (the pure-Go go-ruby-xslt backend).
require "nokogiri"

SOURCE = <<~XML
  <?xml version="1.0"?>
  <catalog>
    <book id="b1"><title>Go</title><price>30</price></book>
    <book id="b2"><title>Ruby</title><price>25</price></book>
  </catalog>
XML

# A stylesheet with a text output method and a caller-overridable parameter.
REPORT = <<~XSL
  <?xml version="1.0"?>
  <xsl:stylesheet version="1.0" xmlns:xsl="http://www.w3.org/1999/XSL/Transform">
    <xsl:output method="text"/>
    <xsl:param name="label">Book</xsl:param>
    <xsl:template match="/">
      <xsl:for-each select="catalog/book"><xsl:value-of select="$label"/>: <xsl:value-of select="title"/> ($<xsl:value-of select="price"/>)
  </xsl:for-each></xsl:template>
  </xsl:stylesheet>
XSL

doc        = Nokogiri::XML(SOURCE)  # parse the source tree once
stylesheet = Nokogiri::XSLT(REPORT) # compile the stylesheet

# apply_to returns the serialized output as a String (xsl:output honoured).
puts stylesheet.apply_to(doc)

# Stylesheet parameters are passed as a Hash and override xsl:param defaults.
puts stylesheet.apply_to(doc, "label" => "Titre")

# An xml-output stylesheet using an attribute-value template ({count(...)}).
xml_ss = Nokogiri::XSLT(<<~XSL)
  <?xml version="1.0"?>
  <xsl:stylesheet version="1.0" xmlns:xsl="http://www.w3.org/1999/XSL/Transform">
    <xsl:output method="xml" indent="yes" omit-xml-declaration="yes"/>
    <xsl:template match="/"><titles count="{count(catalog/book)}"><xsl:for-each select="catalog/book"><t><xsl:value-of select="title"/></t></xsl:for-each></titles></xsl:template>
  </xsl:stylesheet>
XSL

# transform returns a Nokogiri::XML::Document (the result tree) instead.
result = xml_ss.transform(doc)
puts result.class            # => Nokogiri::XML::Document
puts xml_ss.apply_to(doc)    # same transform, serialized to a String
